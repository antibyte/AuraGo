// Chibi Sensei run monitor: the current run (state, elapsed time against the estimate, stage timeline over the run states and the
// trainer stages, live or post-run loss chart, log highlights, cancel with the plain promise, the "stop it on Kaggle" state with
// the notebook link, result card, files, leftovers, delete) and the run history of the project. Live data comes from the window's
// bus (run, runs, metric, stage); the root polls the active run every 2.5 s as a safety net. Pure helpers are exported for the
// Node test. Classic script, publishes P.run.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const PHASES = ['prepare', 'upload', 'start', 'train', 'download', 'cleanup'];
    const PHASE_OF = { queued: 'prepare', packaging: 'prepare', uploading: 'upload', creating_dataset: 'upload', waiting_dataset: 'upload', starting: 'start', running: 'train', downloading: 'download', verifying: 'download', cleaning: 'cleanup' };
    const STAGES = ['boot', 'env_check', 'data', 'install', 'load_base', 'converter', 'base_eval', 'train', 'tuned_eval', 'save', 'gguf', 'manifest'];
    const FAIL_PHASE = { kaggle_submit_unknown: 'start', download_failed: 'download', checksum_mismatch: 'download', status_missing: 'download' };
    const TICK_MS = 1000, EVENTS_KEPT = 200, EVENTS_DEBOUNCE_MS = 1000, METRICS_DEBOUNCE_MS = 5000, CHART_DEBOUNCE_MS = 500;

    const PHASE_KEYS = {
        prepare: 'chibi_sensei.phase.prepare', upload: 'chibi_sensei.phase.upload', start: 'chibi_sensei.phase.start',
        train: 'chibi_sensei.phase.train', download: 'chibi_sensei.phase.download', cleanup: 'chibi_sensei.phase.cleanup'
    };
    const PHASE_STATE_KEYS = {
        done: 'chibi_sensei.phase_state.done', active: 'chibi_sensei.phase_state.active', failed: 'chibi_sensei.phase_state.failed',
        stopped: 'chibi_sensei.phase_state.stopped', todo: 'chibi_sensei.phase_state.todo', skipped: 'chibi_sensei.phase_state.skipped'
    };
    const STAGE_KEYS = {
        boot: 'chibi_sensei.stage.boot', env_check: 'chibi_sensei.stage.env_check', data: 'chibi_sensei.stage.data', install: 'chibi_sensei.stage.install',
        load_base: 'chibi_sensei.stage.load_base', converter: 'chibi_sensei.stage.converter', base_eval: 'chibi_sensei.stage.base_eval',
        train: 'chibi_sensei.stage.train', tuned_eval: 'chibi_sensei.stage.tuned_eval', save: 'chibi_sensei.stage.save',
        gguf: 'chibi_sensei.stage.gguf', manifest: 'chibi_sensei.stage.manifest', unknown: 'chibi_sensei.stage.unknown'
    };
    const RUN_STATE_KEYS = {
        queued: 'chibi_sensei.run_state.queued', packaging: 'chibi_sensei.run_state.packaging', uploading: 'chibi_sensei.run_state.uploading',
        creating_dataset: 'chibi_sensei.run_state.creating_dataset', waiting_dataset: 'chibi_sensei.run_state.waiting_dataset',
        starting: 'chibi_sensei.run_state.starting', running: 'chibi_sensei.run_state.running', downloading: 'chibi_sensei.run_state.downloading',
        verifying: 'chibi_sensei.run_state.verifying', cleaning: 'chibi_sensei.run_state.cleaning', succeeded: 'chibi_sensei.run_state.succeeded',
        failed: 'chibi_sensei.run_state.failed', cancelled: 'chibi_sensei.run_state.cancelled'
    };
    const STAGE_EVENT_KEYS = { start: 'chibi_sensei.event.stage_start', done: 'chibi_sensei.event.stage_done', failed: 'chibi_sensei.event.stage_failed' };
    const FAIL_KEYS = {
        env_no_gpu: { title: 'chibi_sensei.fail.env_no_gpu', fix: 'chibi_sensei.fail.env_no_gpu_fix' },
        env_unsupported_gpu: { title: 'chibi_sensei.fail.env_unsupported_gpu', fix: 'chibi_sensei.fail.env_unsupported_gpu_fix' },
        env_no_internet: { title: 'chibi_sensei.fail.env_no_internet', fix: 'chibi_sensei.fail.env_no_internet_fix' },
        env_low_disk: { title: 'chibi_sensei.fail.env_low_disk', fix: 'chibi_sensei.fail.env_low_disk_fix' },
        env_unsupported_python: { title: 'chibi_sensei.fail.env_unsupported_python', fix: 'chibi_sensei.fail.env_unsupported_python_fix' },
        install_failed: { title: 'chibi_sensei.fail.install_failed', fix: 'chibi_sensei.fail.install_failed_fix' },
        job_invalid: { title: 'chibi_sensei.fail.job_invalid', fix: 'chibi_sensei.fail.job_invalid_fix' },
        data_mismatch: { title: 'chibi_sensei.fail.data_mismatch', fix: 'chibi_sensei.fail.data_mismatch_fix' },
        base_download_failed: { title: 'chibi_sensei.fail.base_download_failed', fix: 'chibi_sensei.fail.base_download_failed_fix' },
        base_load_failed: { title: 'chibi_sensei.fail.base_load_failed', fix: 'chibi_sensei.fail.base_load_failed_fix' },
        oom: { title: 'chibi_sensei.fail.oom', fix: 'chibi_sensei.fail.oom_fix' },
        train_diverged: { title: 'chibi_sensei.fail.train_diverged', fix: 'chibi_sensei.fail.train_diverged_fix' },
        train_failed: { title: 'chibi_sensei.fail.train_failed', fix: 'chibi_sensei.fail.train_failed_fix' },
        eval_failed: { title: 'chibi_sensei.fail.eval_failed', fix: 'chibi_sensei.fail.eval_failed_fix' },
        merge_failed: { title: 'chibi_sensei.fail.merge_failed', fix: 'chibi_sensei.fail.merge_failed_fix' },
        gguf_failed: { title: 'chibi_sensei.fail.gguf_failed', fix: 'chibi_sensei.fail.gguf_failed_fix' },
        gguf_arch_mismatch: { title: 'chibi_sensei.fail.gguf_arch_mismatch', fix: 'chibi_sensei.fail.gguf_arch_mismatch_fix' },
        output_failed: { title: 'chibi_sensei.fail.output_failed', fix: 'chibi_sensei.fail.output_failed_fix' },
        kaggle_submit_unknown: { title: 'chibi_sensei.fail.kaggle_submit_unknown', fix: 'chibi_sensei.fail.kaggle_submit_unknown_fix' },
        remote_timeout: { title: 'chibi_sensei.fail.remote_timeout', fix: 'chibi_sensei.fail.remote_timeout_fix' },
        status_missing: { title: 'chibi_sensei.fail.status_missing', fix: 'chibi_sensei.fail.status_missing_fix' },
        download_failed: { title: 'chibi_sensei.fail.download_failed', fix: 'chibi_sensei.fail.download_failed_fix' },
        checksum_mismatch: { title: 'chibi_sensei.fail.checksum_mismatch', fix: 'chibi_sensei.fail.checksum_mismatch_fix' },
        lora_targets_missing: { title: 'chibi_sensei.fail.lora_targets_missing', fix: 'chibi_sensei.fail.lora_targets_missing_fix' },
        cancelled_by_user: { title: 'chibi_sensei.fail.cancelled_by_user', fix: 'chibi_sensei.fail.cancelled_by_user_fix' },
        desktop_readonly: { title: 'chibi_sensei.fail.desktop_readonly', fix: 'chibi_sensei.fail.desktop_readonly_fix' },
        interrupted: { title: 'chibi_sensei.fail.interrupted', fix: 'chibi_sensei.fail.interrupted_fix' },
        internal: { title: 'chibi_sensei.fail.internal', fix: 'chibi_sensei.fail.internal_fix' },
        unknown: { title: 'chibi_sensei.fail.unknown', fix: 'chibi_sensei.fail.unknown_fix' }
    };
    const WARNING_KEYS = {
        time_budget: 'chibi_sensei.warning.time_budget', base_gguf_skipped: 'chibi_sensei.warning.base_gguf_skipped', unknown: 'chibi_sensei.warning.unknown'
    };
    const ARTIFACT_KEYS = {
        gguf_tuned: 'chibi_sensei.artifact.gguf_tuned', gguf_base: 'chibi_sensei.artifact.gguf_base', adapter: 'chibi_sensei.artifact.adapter',
        adapter_config: 'chibi_sensei.artifact.adapter_config', metrics: 'chibi_sensei.artifact.metrics', eval_samples: 'chibi_sensei.artifact.eval_samples',
        status: 'chibi_sensei.artifact.status', log: 'chibi_sensei.artifact.log', other: 'chibi_sensei.artifact.other'
    };
    const LEFTOVER_KEYS = { dataset: 'chibi_sensei.leftover.dataset', notebook: 'chibi_sensei.leftover.notebook', other: 'chibi_sensei.leftover.other' };
    const PRESET_NAME_KEYS = { quick: 'chibi_sensei.training.preset_quick', balanced: 'chibi_sensei.training.preset_balanced', thorough: 'chibi_sensei.training.preset_thorough' };

    // The phase the run is in now, or was in when it ended. last_state is the server's answer; without it the code, the trainer
    // stage and the remote facts decide.
    function reachedPhase(run) {
        const r = run || {};
        if (PHASE_OF[r.state]) return PHASE_OF[r.state];
        if (r.state === 'succeeded') return 'cleanup';
        if (PHASE_OF[r.last_state]) return PHASE_OF[r.last_state];
        if (FAIL_PHASE[r.error_code]) return FAIL_PHASE[r.error_code];
        if ((r.stage && STAGES.indexOf(r.stage.name) >= 0) || r.remote_started) return 'train';
        const remote = r.remote || {};
        if (remote.notebook) return 'start';
        if (remote.dataset) return 'upload';
        return 'prepare';
    }

    // timeline(run) -> {phase, phases: [{id, status}], stages: [{id, status}]}; status: done | active | failed | stopped | todo | skipped
    function timeline(run) {
        const r = run || {};
        const phase = reachedPhase(r);
        const at = PHASES.indexOf(phase), trainAt = PHASES.indexOf('train');
        const end = r.state === 'failed' ? 'failed' : (r.state === 'cancelled' ? 'stopped' : '');
        const later = end ? 'skipped' : 'todo';
        const phases = PHASES.map((id, i) => ({ id, status: r.state === 'succeeded' || i < at ? 'done' : (i > at ? later : (end || 'active')) }));
        const s = r.stage && STAGES.indexOf(r.stage.name) >= 0 ? r.stage : null;
        const idx = s ? STAGES.indexOf(s.name) : -1;
        const stages = STAGES.map((id, i) => {
            if (r.state === 'succeeded' || at > trainAt) return { id, status: 'done' };
            if (at < trainAt || idx < 0 || i > idx) return { id, status: later };
            if (i < idx || s.state === 'done') return { id, status: 'done' };
            if (s.state === 'failed') return { id, status: 'failed' };
            return { id, status: end || 'active' };
        });
        return { phase, phases, stages };
    }

    // mergeMetrics dedupes by kind and step (the later point wins) and sorts by step, training before validation.
    function mergeMetrics(list, points) {
        const map = new Map();
        (Array.isArray(list) ? list : []).concat(Array.isArray(points) ? points : []).forEach(m => {
            if (m && Number.isFinite(Number(m.step))) map.set((m.kind === 'eval' ? 'eval' : 'train') + ':' + Number(m.step), m);
        });
        return Array.from(map.values()).sort((a, b) => (Number(a.step) - Number(b.step)) || ((a.kind === 'eval') - (b.kind === 'eval')));
    }

    function elapsedSeconds(run, now) {
        const r = run || {};
        const from = Date.parse(r.remote_started || r.created || '');
        if (!Number.isFinite(from)) return 0;
        const to = P.runTerminal(r.state) ? Date.parse(r.finished || r.updated || '') : now;
        return Number.isFinite(to) ? Math.max(0, Math.round((to - from) / 1000)) : 0;
    }

    // liveNote: the key of the note under the chart while the run trains, '' when none applies.
    function liveNote(run, points) {
        const r = run || {};
        if (r.state !== 'running') return '';
        if (r.live === 'no') return 'chibi_sensei.run.live_off';
        if (r.live !== 'yes' && !points) return 'chibi_sensei.run.live_waiting';
        return '';
    }

    function stageName(t, id) { return t(STAGE_KEYS[id] || STAGE_KEYS.unknown); }

    // eventText: one log highlight in words, '' for a shape the UI does not know (never the raw payload).
    function eventText(t, ev) {
        const d = ev && ev.data && typeof ev.data === 'object' ? ev.data : {};
        const str = v => (typeof v === 'string' ? v.slice(0, 300) : '');
        switch (ev && ev.kind) {
        case 'state': { const s = str(d.to) || str(d.state); return RUN_STATE_KEYS[s] ? t('chibi_sensei.event.state', { state: t(RUN_STATE_KEYS[s]) }) : ''; }
        case 'stage': { const key = STAGE_EVENT_KEYS[str(d.state)]; return key ? t(key, { stage: stageName(t, str(d.stage)) }) : ''; }
        case 'warn': return t('chibi_sensei.event.warn', { text: WARNING_KEYS[str(d.code)] ? t(WARNING_KEYS[str(d.code)]) : (str(d.message) || t(WARNING_KEYS.unknown)) });
        case 'error': return t('chibi_sensei.event.error', { text: FAIL_KEYS[str(d.code)] ? t(FAIL_KEYS[str(d.code)].title) : (str(d.message) || t(FAIL_KEYS.unknown.title)) });
        case 'kaggle': return str(d.step) ? t('chibi_sensei.event.kaggle', { step: str(d.step) }) : '';
        case 'log': return str(d.text);
        default: return '';
        }
    }

    // failText: the title and the next step for a run failure code; a code the UI does not know gets the generic pair.
    function failText(t, code) {
        const keys = FAIL_KEYS[code] || FAIL_KEYS.unknown;
        return { title: t(keys.title), fix: t(keys.fix) };
    }

    // confusion(part, which) -> {labels, rows: [{label, cells}]} or null. Each row is the expected label; the cells follow the
    // labels and one more column counts answers that match no label.
    function confusion(part, which) {
        const labels = part && Array.isArray(part.labels) ? part.labels.map(l => String(l).slice(0, 40)) : [];
        const grid = part && part.confusion && Array.isArray(part.confusion[which]) ? part.confusion[which] : null;
        if (!labels.length || !grid) return null;
        const rows = labels.map((label, i) => {
            const row = Array.isArray(grid[i]) ? grid[i] : [];
            return { label, cells: labels.concat(['']).map((_, j) => Math.max(0, Math.round(Number(row[j]) || 0))) };
        });
        return { labels, rows };
    }

    // number-or-missing formatters for report values that may be null
    const isNum = v => v !== null && v !== undefined && v !== '' && Number.isFinite(Number(v));
    const orMissing = (t, v, fn) => (isNum(v) ? fn(Number(v)) : t('chibi_sensei.run.value_missing'));

    function createArea(inst) {
        const { el } = P;
        const t = inst.t;
        let disposed = false, selectedId = '', pinned = false, detail = null, detailSeq = 0, cancelling = false, deleting = false, logFor = '';
        let chartTimer = 0, eventsTimer = 0, metricsTimer = 0;
        const sigs = {};
        const chart = P.chart ? P.chart.create(inst) : null;

        // ---- nodes ----
        const heading = el('h3', { class: 'csn-run-title', tabindex: '-1', dataset: { csn: 'run-title' } });
        const badge = el('span', { class: 'csn-badge csn-run-state', dataset: { csn: 'run-state', state: '' } });
        const status = el('p', { class: 'csn-run-status', role: 'status', dataset: { csn: 'run-status' } });
        const timeLine = el('p', { class: 'csn-run-meta', dataset: { csn: 'run-time' } });
        const progress = el('p', { class: 'csn-run-meta', dataset: { csn: 'run-progress' } });
        const liveNoteNode = el('p', { class: 'csn-note', dataset: { csn: 'run-live-note' } });
        const longer = el('p', { class: 'csn-note', dataset: { csn: 'run-longer', tone: 'warn' } });
        const phaseList = el('ol', { class: 'csn-timeline', ariaLabel: t('chibi_sensei.run.timeline'), dataset: { csn: 'run-timeline' } });
        const stageList = el('ol', { class: 'csn-stages', ariaLabel: t('chibi_sensei.run.stages'), dataset: { csn: 'run-stages' } });
        const cancelBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'run-cancel' }, text: t('chibi_sensei.run.cancel'), onClick: () => cancelRun() });
        const deleteBtn = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: 'run-delete' }, text: t('chibi_sensei.run.delete'), onClick: () => deleteRun() });
        const actions = el('div', { class: 'csn-actions', dataset: { csn: 'run-actions' } }, cancelBtn, deleteBtn);
        const cancelNote = el('p', { class: 'csn-note', dataset: { csn: 'run-cancel-note' } });
        const stopBox = el('div', { class: 'csn-note', dataset: { csn: 'run-stop', tone: 'warn' } });
        const result = el('div', { class: 'csn-result', dataset: { csn: 'run-result', outcome: '' } });
        const files = el('div', { dataset: { csn: 'run-files' } });
        const leftovers = el('div', { dataset: { csn: 'run-leftovers' } });
        const eventList = el('ol', { class: 'csn-events' });
        const eventsBox = el('details', { class: 'csn-advanced', dataset: { csn: 'run-events' } }, el('summary', { text: t('chibi_sensei.run.events_title') }), eventList);
        const jobsBody = el('div');
        const jobsBox = el('details', { class: 'csn-advanced', dataset: { csn: 'run-jobs' } }, el('summary', { text: t('chibi_sensei.run.jobs_title') }), jobsBody);
        const logText = el('pre', { class: 'csn-log', dataset: { csn: 'run-log-text' } });
        const logBox = el('details', { class: 'csn-advanced', dataset: { csn: 'run-log' } }, el('summary', { text: t('chibi_sensei.run.log_title') }), logText);
        const errorLine = el('p', { class: 'csn-error', role: 'alert', dataset: { csn: 'run-error' } });
        const current = el('section', { class: 'csn-training-section csn-run', dataset: { csn: 'run-current' } },
            el('div', { class: 'csn-run-head' }, heading, badge), status, timeLine, progress, liveNoteNode, longer, phaseList, stageList, actions, cancelNote, stopBox,
            result, chart ? chart.el : null, files, leftovers, eventsBox, jobsBox, logBox, errorLine);
        const historyList = el('ol', { class: 'csn-run-list' });
        const history = el('section', { class: 'csn-training-section csn-run-history', dataset: { csn: 'run-history' } }, el('h3', { text: t('chibi_sensei.run.history_title') }), historyList);
        const node = el('div', { class: 'csn-runs', dataset: { csn: 'runs' } }, current, history);
        [liveNoteNode, longer, stageList, cancelNote, stopBox, result, files, leftovers, errorLine].forEach(n => { n.hidden = true; });
        current.hidden = true;

        const tolerant = promise => promise.catch(err => { if (err && err.aborted) throw err; return null; });
        const stale = mine => disposed || mine !== detailSeq;
        const showError = text => { errorLine.textContent = text || ''; errorLine.hidden = !text; };
        const setText = (node2, text) => { if (node2.textContent !== text) node2.textContent = text; };
        // Rebuilds a box only when its inputs changed, so selected text and focus survive the frequent live updates.
        const rebuild = (name, box, sig, build) => {
            if (sigs[name] === sig) return;
            sigs[name] = sig;
            P.clear(box);
            build(box);
        };

        function pickSelected() {
            const runs = inst.runs || [];
            const active = P.activeRun(runs);
            if (active && !(pinned && runs.some(r => r.id === selectedId))) return active.id;
            if (selectedId && runs.some(r => r.id === selectedId)) return selectedId;
            return runs.length ? runs[0].id : '';
        }

        function sync() {
            const next = pickSelected();
            paintHistory();
            if (next === selectedId) return;
            selectedId = next;
            if (next) loadDetail(next); else { detail = null; detailSeq++; paintDetail(); }
        }

        async function loadDetail(id) {
            const mine = ++detailSeq;
            detail = { id, run: (inst.runs || []).find(r => r.id === id) || { id }, metrics: [], maxSteps: 0, live: '', events: [], artifacts: null, report: null };
            showError('');
            paintDetail();
            try {
                const [run, metrics, events] = await Promise.all([inst.api.getRun(id), tolerant(inst.api.runMetrics(id, 0)), tolerant(inst.api.runEvents(id, 0, EVENTS_KEPT))]);
                if (stale(mine)) return;
                detail.run = run;
                applyMetricsBody(metrics, true);
                detail.events = (events || []).slice(-EVENTS_KEPT);
                if (P.runTerminal(run.state)) await loadFinal(id, mine);
                if (!stale(mine)) paintDetail();
            } catch (err) {
                if (stale(mine) || (err && err.aborted)) return;
                const e = P.errors.normalize(err);
                if (e.status === 404 || e.code === 'run_not_found') { inst.removeRun(id); return; }
                showError(inst.handleError(err));
            }
        }

        async function loadFinal(id, mine) {
            const [artifacts, report] = await Promise.all([tolerant(inst.api.runArtifacts(id)), tolerant(inst.api.runReport(id))]);
            if (stale(mine)) return;
            detail.artifacts = artifacts || [];
            detail.report = report || null;
        }

        // The run just ended: the post-run metrics replace the live ones, then files, report and the last events.
        async function finishDetail(id) {
            const mine = detailSeq;
            try {
                const [run, metrics] = await Promise.all([inst.api.getRun(id), tolerant(inst.api.runMetrics(id, 0))]);
                if (stale(mine)) return;
                detail.run = run;
                applyMetricsBody(metrics, true);
                await loadFinal(id, mine);
                await loadEvents(id, mine);
            } catch (err) { if (!stale(mine) && !(err && err.aborted)) showError(inst.handleError(err)); }
            if (!stale(mine)) paintDetail();
        }

        function applyMetricsBody(body, replace) {
            if (!body || !detail) return;
            detail.metrics = mergeMetrics(replace ? [] : detail.metrics, body.metrics);
            if (Number(body.max_steps) > 0) detail.maxSteps = Number(body.max_steps);
            if (typeof body.live === 'string') detail.live = body.live;
        }

        async function loadEvents(id, mine) {
            if (!detail || detail.id !== id) return;
            const after = detail.events.length ? Number(detail.events[detail.events.length - 1].id) || 0 : 0;
            try {
                const list = await inst.api.runEvents(id, after, EVENTS_KEPT);
                if (stale(mine)) return;
                detail.events = detail.events.concat(list.filter(e => e && Number(e.id) > after)).slice(-EVENTS_KEPT);
                paintEvents();
            } catch (_) { /* the next stage event or the end of the run loads them */ }
        }

        // Without the event stream the chart would wait for the end: when the run reports a step past the last point, fetch the rest.
        // The metrics route is inclusive (step >= since), so asking from the last known step repeats that point and replaces it.
        function maybeFetchMetrics() {
            const r = detail && detail.run;
            if (!r || r.state !== 'running' || !r.stage || !(Number(r.stage.step) > 0)) return;
            const last = detail.metrics.length ? Number(detail.metrics[detail.metrics.length - 1].step) : 0;
            if (Number(r.stage.step) <= last || metricsTimer) return;
            const id = detail.id, mine = detailSeq;
            metricsTimer = setTimeout(async () => {
                metricsTimer = 0;
                try { const body = await inst.api.runMetrics(id, last); if (!stale(mine)) { applyMetricsBody(body, false); scheduleChart(); } } catch (_) { /* next time */ }
            }, METRICS_DEBOUNCE_MS);
        }

        function scheduleChart() {
            if (chartTimer) return;
            chartTimer = setTimeout(() => { chartTimer = 0; if (!disposed) paintChart(); }, CHART_DEBOUNCE_MS);
        }

        function onRun(run) {
            if (disposed || !run) return;
            sync();
            if (!detail || run.id !== detail.id) return;
            const was = P.runTerminal(detail.run && detail.run.state);
            detail.run = Object.assign({}, detail.run || {}, run);
            if (!was && P.runTerminal(detail.run.state)) { paintDetail(); finishDetail(detail.id); return; }
            maybeFetchMetrics();
            paintDetail();
        }
        function onMetric(body) {
            if (disposed || !detail || !body || body.run_id !== detail.id) return;
            detail.metrics = mergeMetrics(detail.metrics, body.points);
            if (detail.live !== 'no') detail.live = 'yes';
            scheduleChart();
        }
        function onStage(body) {
            if (disposed || !detail || !body || body.run_id !== detail.id) return;
            const id = detail.id, mine = detailSeq;
            clearTimeout(eventsTimer);
            eventsTimer = setTimeout(() => loadEvents(id, mine), EVENTS_DEBOUNCE_MS);
        }

        // ---- cancel and delete ----
        async function cancelRun() {
            const run = detail && detail.run;
            if (!run || P.runTerminal(run.state) || run.cancel_requested || cancelling) return;
            const minutes = P.format.minutes(run.session_timeout_minutes || (run.cancel && run.cancel.time_limit_minutes));
            const ok = await inst.askConfirm(t('chibi_sensei.run.cancel_title'), t('chibi_sensei.run.cancel_text', { minutes }));
            if (!ok || disposed) return;
            cancelling = true; paintActions();
            try { const updated = await inst.api.cancelRun(run.id); if (!disposed) inst.applyRun(updated); }
            catch (err) { if (!disposed && !(err && err.aborted)) showError(inst.handleError(err)); }
            finally { cancelling = false; if (!disposed) paintActions(); }
        }

        async function deleteRun() {
            const run = detail && detail.run;
            if (!run || !P.runTerminal(run.state) || inst.readonly || deleting) return;
            const ok = await inst.askConfirm(t('chibi_sensei.run.delete_title'), t('chibi_sensei.run.delete_text'));
            if (!ok || disposed) return;
            deleting = true; paintActions();
            try { await inst.api.deleteRun(run.id); if (disposed) return; pinned = false; inst.removeRun(run.id); inst.notify(t('chibi_sensei.run.deleted'), 'info'); }
            catch (err) { if (!disposed && !(err && err.aborted)) showError(inst.handleError(err)); }
            finally { deleting = false; if (!disposed) paintActions(); }
        }

        // ---- painters ----
        function paintHistory() {
            const runs = inst.runs || [];
            P.clear(historyList);
            if (!runs.length) { historyList.appendChild(el('li', null, el('p', { class: 'csn-muted', text: t('chibi_sensei.run.history_empty') }))); return; }
            runs.forEach(r => {
                const date = P.format.dateTime(r.created);
                const preset = PRESET_NAME_KEYS[r.preset] ? t(PRESET_NAME_KEYS[r.preset]) : '';
                const state = RUN_STATE_KEYS[r.state] ? t(RUN_STATE_KEYS[r.state]) : '';
                historyList.appendChild(el('li', null, el('button', {
                    type: 'button', class: 'csn-run-item', ariaPressed: r.id === selectedId ? 'true' : 'false', ariaLabel: t('chibi_sensei.run.item_label', { date, preset, state }),
                    dataset: { csn: 'run-item', run: r.id, state: r.state },
                    onClick: () => { pinned = true; if (r.id !== selectedId) { selectedId = r.id; loadDetail(r.id); } paintHistory(); }
                }, el('span', { text: date }), el('span', { text: preset }), el('span', { class: 'csn-run-item-state', text: state }))));
            });
        }

        function paintHead() {
            const run = detail.run;
            const terminal = P.runTerminal(run.state);
            setText(heading, terminal ? t('chibi_sensei.run.title_past', { date: P.format.dateTime(run.created) }) : t('chibi_sensei.run.title'));
            const word = RUN_STATE_KEYS[run.state] ? t(RUN_STATE_KEYS[run.state]) : '';
            setText(badge, word);
            badge.dataset.state = run.state || '';
            let sentence = word;
            const remoteStatus = run.remote && run.remote.status;
            if (run.state === 'running' && remoteStatus) sentence += ' ' + t('chibi_sensei.run.status_kaggle', { status: String(remoteStatus).slice(0, 60) });
            setText(status, sentence);
        }

        function paintTime() {
            if (!detail || !detail.run) return;
            const run = detail.run;
            const terminal = P.runTerminal(run.state);
            const secs = elapsedSeconds(run, Date.now());
            let line = t('chibi_sensei.run.elapsed', { time: P.format.duration(secs) });
            const estimate = Number(run.estimate_minutes);
            if (estimate > 0) line += ' · ' + t('chibi_sensei.run.estimate', { minutes: P.format.minutes(estimate) });
            setText(timeLine, line);
            const over = !terminal && estimate > 0 && secs > 1.25 * estimate * 60;
            setText(longer, over ? t('chibi_sensei.run.longer', { minutes: P.format.minutes(run.session_timeout_minutes) }) : '');
            longer.hidden = !over;
            const s = run.stage;
            let prog = '';
            if (!terminal && s && s.name === 'train' && Number(s.max_steps) > 0) {
                prog = t('chibi_sensei.run.progress', { step: P.format.number(s.step), max: P.format.number(s.max_steps), epoch: P.format.decimal(s.epoch, 1) });
                if (Number(s.eta_s) > 0) prog += ' · ' + t('chibi_sensei.run.eta', { time: P.format.duration(s.eta_s) });
            }
            setText(progress, prog);
            progress.hidden = !prog;
        }

        function paintTimeline() {
            const run = detail.run;
            const tl = timeline(run);
            const mark = (list, items, keyOf, titled) => {
                if (list.children.length !== items.length) {
                    P.clear(list);
                    items.forEach(() => list.appendChild(el('li')));
                }
                items.forEach((item, i) => {
                    const li = list.children[i];
                    const name = t(keyOf(item.id));
                    const word = t(PHASE_STATE_KEYS[item.status]);
                    li.dataset.status = item.status;
                    if (titled) { li.className = 'csn-stage'; li.dataset.stage = item.id; li.title = word; setText(li, name); li.setAttribute('aria-label', name + ', ' + word); } else {
                        li.className = 'csn-phase'; li.dataset.phase = item.id;
                        if (!li.firstChild) { li.appendChild(el('span', { class: 'csn-phase-name' })); li.appendChild(el('span', { class: 'csn-phase-state' })); }
                        setText(li.firstChild, name); setText(li.lastChild, word);
                        li.setAttribute('aria-label', name + ', ' + word);
                    }
                    if (item.status === 'active') li.setAttribute('aria-current', 'step'); else li.removeAttribute('aria-current');
                });
            };
            mark(phaseList, tl.phases, id => PHASE_KEYS[id], false);
            mark(stageList, tl.stages, id => STAGE_KEYS[id], true);
            stageList.hidden = PHASES.indexOf(tl.phase) < PHASES.indexOf('train');
        }

        function paintActions() {
            const run = detail && detail.run;
            const terminal = !!run && P.runTerminal(run.state);
            cancelBtn.hidden = !run || terminal || !!run.cancel_requested;
            cancelBtn.disabled = cancelling;
            setText(cancelBtn, cancelling ? t('chibi_sensei.pending') : t('chibi_sensei.run.cancel'));
            deleteBtn.hidden = !terminal;
            deleteBtn.disabled = inst.readonly || deleting;
            deleteBtn.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
            actions.hidden = cancelBtn.hidden && deleteBtn.hidden;
        }

        function paintCancel() {
            const run = detail.run;
            const pending = !!run.cancel_requested && !P.runTerminal(run.state);
            const cancel = run.cancel || {};
            const noNotebook = !(run.remote && run.remote.notebook);
            const note = pending ? (noNotebook ? t('chibi_sensei.run.stopped_local') : t('chibi_sensei.run.cancel_requested', { minutes: P.format.minutes(cancel.time_limit_minutes || run.session_timeout_minutes) })) : '';
            setText(cancelNote, note);
            cancelNote.hidden = !note;
            const showStop = pending && cancel.remote_stopped === false && !!cancel.stop_url;
            stopBox.hidden = !showStop;
            rebuild('stop', stopBox, showStop ? String(cancel.stop_url) : '', box => {
                if (!showStop) return;
                box.appendChild(el('h4', { text: t('chibi_sensei.run.stop_title') }));
                box.appendChild(el('p', { text: t('chibi_sensei.run.stop_text') }));
                box.appendChild(P.linkRow(t, cancel.stop_url, 'run-stop-link'));
            });
        }

        function paintResult() {
            const run = detail.run;
            const terminal = P.runTerminal(run.state);
            result.hidden = !terminal;
            if (!terminal) { rebuild('result', result, '', () => {}); return; }
            result.dataset.outcome = run.state;
            const sig = JSON.stringify([run.state, run.error_code, run.error, run.warning_code, run.result, detail.report]);
            rebuild('result', result, sig, box => {
                const line = (text, name) => box.appendChild(el('p', { text, dataset: name ? { csn: name } : null }));
                if (run.state === 'succeeded') fillSuccess(box, line);
                else if (run.state === 'failed') {
                    const f = failText(t, run.error_code);
                    box.appendChild(el('h4', { text: t('chibi_sensei.run.failed_title') }));
                    line(f.title, 'run-fail');
                    box.appendChild(el('h4', { text: t('chibi_sensei.run.fix_title') }));
                    line(f.fix, 'run-fix');
                    if (run.error) box.appendChild(el('code', { class: 'csn-detail', text: String(run.error).slice(0, 300) }));
                } else line(t('chibi_sensei.run.cancelled_text'));
            });
        }

        function fillSuccess(box, line) {
            const run = detail.run, res = run.result || {}, rep = detail.report || {};
            const dec = v => P.format.decimal(v, 3);
            const miss = v => orMissing(t, v, dec);
            const share = v => orMissing(t, v, P.format.share);
            box.appendChild(el('h4', { text: t('chibi_sensei.run.result_title') }));
            if (isNum(res.steps)) line(t('chibi_sensei.run.result_steps', { steps: P.format.number(res.steps), epochs: P.format.decimal(res.epochs, 1) }));
            if (isNum(res.train_loss)) line(t('chibi_sensei.run.train_loss', { value: dec(res.train_loss) }));
            const loss = rep.loss || { base: res.eval_loss_base, tuned: res.eval_loss_tuned };
            if (isNum(loss.base) || isNum(loss.tuned)) {
                line(t('chibi_sensei.run.eval_loss', { base: miss(loss.base), tuned: miss(loss.tuned) }));
                line(t('chibi_sensei.run.loss_help'));
            }
            if (rep.exact_match) line(t('chibi_sensei.run.exact', { base: share(rep.exact_match.base), tuned: share(rep.exact_match.tuned), n: P.format.number(rep.exact_match.n) }));
            if (rep.label_accuracy) {
                const acc = rep.label_accuracy;
                line(t('chibi_sensei.run.labels', { base: share(acc.base), tuned: share(acc.tuned), n: P.format.number(acc.n) }));
                const grid = confusionBox(acc);
                if (grid) box.appendChild(grid);
            }
            if (rep.perplexity && (isNum(rep.perplexity.base) || isNum(rep.perplexity.tuned))) line(t('chibi_sensei.run.perplexity', { base: miss(rep.perplexity.base), tuned: miss(rep.perplexity.tuned) }));
            if (rep.small_sample) line(t('chibi_sensei.run.small_sample', { n: P.format.number(rep.samples) }));
            if (res.gpu) line(t('chibi_sensei.run.gpu_used', { name: String(res.gpu).slice(0, 80) }));
            if (run.warning_code) {
                const key = WARNING_KEYS[run.warning_code] || WARNING_KEYS.unknown;
                box.appendChild(el('div', { class: 'csn-note', dataset: { csn: 'run-warning', tone: 'warn' } }, el('strong', { text: t('chibi_sensei.run.warning_title') }), el('p', { text: t(key) })));
            }
        }

        // The label table: one block per model, rows are the expected labels, the last column counts answers that match no label.
        function confusionBox(acc) {
            const parts = [['base', 'chibi_sensei.run.confusion_base'], ['tuned', 'chibi_sensei.run.confusion_tuned']].map(pair => ({ data: confusion(acc, pair[0]), title: t(pair[1]) })).filter(p => p.data);
            if (!parts.length) return null;
            const box = el('details', { class: 'csn-advanced', dataset: { csn: 'run-confusion' } }, el('summary', { text: t('chibi_sensei.run.confusion_title') }), el('p', { class: 'csn-muted', text: t('chibi_sensei.run.confusion_help') }));
            parts.forEach(part => {
                const head = el('tr', null, el('th', { scope: 'col', text: t('chibi_sensei.run.confusion_expected') }));
                part.data.labels.forEach(l => head.appendChild(el('th', { scope: 'col', text: l })));
                head.appendChild(el('th', { scope: 'col', dataset: { csn: 'confusion-none' }, text: t('chibi_sensei.run.confusion_none') }));
                const body = el('tbody');
                part.data.rows.forEach(row => body.appendChild(el('tr', null, el('th', { scope: 'row', text: row.label }), row.cells.map(c => el('td', { text: P.format.number(c) })))));
                box.appendChild(el('div', { class: 'csn-confusion-wrap' }, el('table', { class: 'csn-confusion', dataset: { csn: 'confusion' } }, el('caption', { text: part.title }), el('thead', null, head), body)));
            });
            return box;
        }

        function paintChart() {
            if (!chart || !detail) return;
            const key = liveNote(detail.run, detail.metrics.length);
            chart.update(detail.metrics, detail.maxSteps, key ? t(key) : '');
        }

        function paintFiles() {
            const run = detail.run;
            const show = P.runTerminal(run.state) && Array.isArray(detail.artifacts);
            files.hidden = !show;
            rebuild('files', files, show ? run.id + JSON.stringify(detail.artifacts) : '', box => {
                if (!show) return;
                box.appendChild(el('h4', { text: t('chibi_sensei.run.files_title') }));
                if (!detail.artifacts.length) { box.appendChild(el('p', { class: 'csn-muted', text: t('chibi_sensei.run.files_empty') })); return; }
                box.appendChild(el('ul', { class: 'csn-files' }, detail.artifacts.map(a => el('li', { class: 'csn-file', dataset: { csn: 'artifact', kind: a.kind } },
                    el('span', { class: 'csn-file-name', text: String(a.name || '') }),
                    el('span', { class: 'csn-muted', text: t(ARTIFACT_KEYS[a.kind] || ARTIFACT_KEYS.other) }),
                    el('span', { class: 'csn-muted', text: P.format.bytes(a.bytes) }),
                    el('a', { class: 'csn-btn', href: inst.api.artifactURL(run.id, a.id), download: true, ariaLabel: t('chibi_sensei.run.download_label', { name: String(a.name || '') }), dataset: { csn: 'artifact-download' }, text: t('chibi_sensei.run.download') })))));
            });
        }

        function paintLeftovers() {
            const list = Array.isArray(detail.run.leftovers) ? detail.run.leftovers : [];
            leftovers.hidden = !list.length;
            rebuild('leftovers', leftovers, JSON.stringify(list), box => {
                if (!list.length) return;
                box.appendChild(el('h4', { text: t('chibi_sensei.run.leftovers_title') }));
                box.appendChild(el('p', { text: t('chibi_sensei.run.leftovers_text') }));
                list.forEach((item, i) => box.appendChild(el('div', { class: 'csn-leftover', dataset: { csn: 'leftover', kind: item.kind } },
                    el('strong', { text: t(LEFTOVER_KEYS[item.kind] || LEFTOVER_KEYS.other) }), ' ', el('code', { text: String(item.ref || '').slice(0, 200) }), P.linkRow(t, item.url, 'leftover-' + i))));
            });
        }

        function paintEvents() {
            if (!detail) return;
            P.clear(eventList);
            detail.events.forEach(ev => {
                const text = eventText(t, ev);
                if (!text) return;
                eventList.appendChild(el('li', { class: 'csn-event', dataset: { kind: ev.kind } }, el('span', { class: 'csn-event-time', text: P.format.dateTime(ev.ts) }), el('span', { text })));
            });
            if (!eventList.firstChild) eventList.appendChild(el('li', { class: 'csn-muted', text: t('chibi_sensei.run.events_empty') }));
        }

        function paintJobs() {
            const remote = detail.run.remote || {};
            const jobs = Array.isArray(remote.kaggle_jobs) ? remote.kaggle_jobs : [];
            rebuild('jobs', jobsBody, JSON.stringify([remote.notebook_url, jobs]), box => {
                if (remote.notebook_url) {
                    box.appendChild(el('p', { text: t('chibi_sensei.run.notebook_title') }));
                    box.appendChild(P.linkRow(t, remote.notebook_url, 'run-notebook'));
                }
                if (jobs.length) box.appendChild(el('ul', null, jobs.map(j => el('li', { text: t('chibi_sensei.run.job_line', { step: String(j.step || '').slice(0, 60), id: String(j.job_id || '').slice(0, 60) }) }))));
            });
            jobsBox.hidden = !jobsBody.firstChild;
        }

        async function loadLog() {
            if (!detail || !logBox.open || logFor === detail.id) return;
            const id = detail.id, mine = detailSeq;
            logFor = id;
            setText(logText, t('chibi_sensei.pending'));
            try {
                const body = await inst.api.runLog(id);
                if (stale(mine)) return;
                setText(logText, (body && typeof body.tail === 'string' ? body.tail : '') || t('chibi_sensei.run.log_empty'));
            } catch (err) {
                if (stale(mine) || (err && err.aborted)) return;
                logFor = '';
                setText(logText, '');
                showError(inst.handleError(err));
            }
        }
        logBox.addEventListener('toggle', loadLog);

        function paintDetail() {
            if (disposed) return;
            current.hidden = !detail;
            if (!detail) { paintActions(); return; }
            if (logFor && logFor !== detail.id) { logFor = ''; setText(logText, ''); }
            paintHead();
            paintTime();
            paintTimeline();
            paintActions();
            paintCancel();
            paintResult();
            paintChart();
            paintFiles();
            paintLeftovers();
            paintEvents();
            paintJobs();
            loadLog();
            paintHistory();
        }

        const offs = [inst.bus.on('run', onRun), inst.bus.on('runs', sync), inst.bus.on('metric', onMetric), inst.bus.on('stage', onStage), inst.bus.on('readonly', () => paintActions())];
        const ticker = setInterval(() => { if (!disposed && detail && detail.run && !P.runTerminal(detail.run.state)) paintTime(); }, TICK_MS);
        sync();

        return {
            el: node,
            focus() { if (!current.hidden) { current.scrollIntoView({ block: 'start' }); heading.focus({ preventScroll: true }); } },
            dispose() {
                disposed = true;
                clearInterval(ticker); clearTimeout(chartTimer); clearTimeout(eventsTimer); clearTimeout(metricsTimer);
                offs.forEach(off => { if (typeof off === 'function') off(); });
                if (chart) chart.dispose();
            }
        };
    }

    P.run = { createArea, timeline, reachedPhase, mergeMetrics, elapsedSeconds, liveNote, eventText, failText, confusion, PHASES, STAGES };
})();
