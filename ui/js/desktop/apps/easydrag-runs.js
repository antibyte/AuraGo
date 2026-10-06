// EasyDrag runs: test dialog (trigger, sample data, real-effects warning), live run stream
// (SSE with reconnect), step and wire states, run drawer and read-only run view.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const EFFECTS_KEY = 'aurago.easydrag.effects-ok.';
    const EFFECT_ICONS = { sends_message: 'brand-telegram', writes_files: 'file-pencil', controls_devices: 'home', runs_code: 'api', deletes: 'trash', system_change: 'settings' };

    function triggers(ed) {
        return ed.model.doc.nodes.filter(n => { const i = ed.model.info(n.type); return i && i.trigger && !n.settings.disabled; });
    }

    // effects lists the outward effects of enabled nodes (for the real-effects warning).
    function effects(ed, onlyNode) {
        const out = new Map();
        ed.model.doc.nodes.forEach(n => {
            if (n.settings.disabled || (onlyNode && n.id !== onlyNode)) return;
            const i = ed.model.info(n.type);
            (i && i.effects || []).forEach(effect => {
                if (!out.has(effect)) out.set(effect, []);
                out.get(effect).push(n.label || n.type);
            });
        });
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
        let drawer = null;
        let drawerFilter = 'all';

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

        function rememberRun(run, steps) {
            const label = run.mode === 'test'
                ? t('easydrag.ui.input_last_test', { time: core.fmt.relative(run.started_at) })
                : t('easydrag.ui.input_last_live', { time: core.fmt.relative(run.started_at) });
            ed.lastRunData = { runId: run.id, label, roots: rootsFrom(steps, run) };
        }

        // ── stream ──────────────────────────────────────────────────────────────

        function closeStream() {
            clearTimeout(reconnectTimer);
            if (source) { source.close(); source = null; }
        }

        function attach(runId, meta) {
            closeStream();
            lastSeq = 0;
            setRun({ id: runId, status: 'queued', mode: (meta && meta.mode) || 'test', steps: new Map(), record: null, error: '' });
            connect(runId);
        }

        function connect(runId) {
            if (!ed.run || ed.run.id !== runId) return;
            const es = new EventSource(ed.api.eventsUrl(runId, lastSeq || undefined));
            source = es;
            es.addEventListener('snapshot', (event) => {
                const detail = JSON.parse(event.data);
                if (!ed.run || ed.run.id !== runId) return;
                ed.run.record = detail.run;
                ed.run.status = detail.run.status;
                (detail.steps || []).forEach(s => ed.run.steps.set(s.node_id, s));
                setRun(ed.run);
            });
            es.addEventListener('event', (event) => {
                const ev = JSON.parse(event.data);
                if (!ed.run || ed.run.id !== runId) return;
                lastSeq = Math.max(lastSeq, ev.seq || 0);
                applyEvent(ev);
            });
            es.addEventListener('end', () => { es.close(); source = null; finish(runId); });
            es.onerror = () => {
                es.close();
                source = null;
                if (!ed.run || ed.run.id !== runId || isFinal(ed.run.status)) return;
                reconnectTimer = setTimeout(() => connect(runId), 1500);
            };
        }

        function isFinal(status) { return ['success', 'error', 'cancelled'].includes(status); }

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

        async function finish(runId) {
            if (!ed.run || ed.run.id !== runId) return;
            try {
                const detail = await ed.api.run(runId, false);
                ed.run.record = detail.run;
                ed.run.status = detail.run.status;
                (detail.steps || []).forEach(s => ed.run.steps.set(s.node_id, s));
                rememberRun(detail.run, detail.steps || []);
            } catch (err) { /* keep the streamed state */ }
            setRun(ed.run);
            const failed = Array.from(ed.run.steps.values()).find(s => s.status === 'error');
            const node = failed && ed.model.node(failed.node_id);
            canvas.announce(ed.run.status === 'success' ? t('easydrag.ui.run_done') : node ? t('easydrag.ui.run_failed_at', { node: node.label }) : t('easydrag.ui.run_failed'));
            if (drawer) loadRuns();
        }

        // ── starting runs ───────────────────────────────────────────────────────

        function effectsConfirmed() { return !!core.storage.get(EFFECTS_KEY + ed.flow.id, false) || !!ed.effectsConfirmed; }

        function effectsMarkup(list) {
            if (!list.size) return '';
            return '<div class="ed-callout ed-callout--warn">' + core.icon('alert') + '<div><strong>' + esc(t('easydrag.ui.effects_title')) + '</strong><ul class="ed-effects">' +
                Array.from(list.entries()).map(([effect, nodes]) => '<li>' + core.icon(EFFECT_ICONS[effect] || 'bolt') + '<span>' + esc(core.tr(t, 'easydrag.ui.effect_' + effect, effect)) + '</span><span class="ed-muted">' + esc(nodes.join(', ')) + '</span></li>').join('') +
                '</ul><label class="ed-check"><input type="checkbox" data-ed-effects-skip> ' + esc(t('easydrag.ui.effects_remember')) + '</label></div></div>';
        }

        async function startTest(opts) {
            if (ed.readonly || ed.runView) return;
            const o = opts || {};
            if (ed.saver) await ed.saver.flush();
            const list = triggers(ed);
            if (!list.length) { ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_no_trigger'), type: 'error' }); return; }
            const remembered = core.storage.get('aurago.easydrag.test-trigger.' + ed.flow.id, '');
            let trigger = list.find(n => n.id === (o.triggerNode || remembered)) || list.find(n => n.type === 'trigger.manual') || list[0];
            const fx = effects(ed, o.onlyNode);
            const needConfirm = fx.size && !effectsConfirmed();
            if (o.quick && !needConfirm) { await run(trigger.id, null, o.onlyNode, false); return; }
            let sample = {};
            try { sample = (await ed.api.testData(ed.flow.id, trigger.id)).data || {}; } catch (err) { sample = {}; }
            const dialog = core.modal(ed.root, {
                title: o.onlyNode ? t('easydrag.ui.test_node_title') : t('easydrag.ui.test_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--test',
                body: (list.length > 1 ? '<label class="ed-label">' + esc(t('easydrag.ui.test_trigger')) + '<select class="ed-input" data-ed-test-trigger>' +
                    list.map(n => '<option value="' + esc(n.id) + '"' + (n.id === trigger.id ? ' selected' : '') + '>' + esc(n.label) + '</option>').join('') + '</select></label>' : '') +
                    '<label class="ed-label">' + esc(t('easydrag.ui.test_data')) + '<textarea class="ed-input ed-code" rows="9" spellcheck="false" data-ed-test-data>' + esc(JSON.stringify(sample, null, 2)) + '</textarea></label>' +
                    '<p class="ed-hint">' + esc(t('easydrag.ui.test_data_hint')) + '</p><p class="ed-error" hidden></p>' +
                    '<label class="ed-check"><input type="checkbox" data-ed-test-remember checked> ' + esc(t('easydrag.ui.test_remember')) + '</label>' +
                    (needConfirm ? effectsMarkup(fx) : ''),
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'run', label: t('easydrag.ui.test_run'), primary: true, icon: 'play' }],
                onAction: async (action, d) => {
                    if (action !== 'run') return true;
                    const err = d.body.querySelector('.ed-error');
                    let data;
                    try { data = JSON.parse(d.body.querySelector('[data-ed-test-data]').value || '{}'); } catch (e) { err.hidden = false; err.textContent = t('easydrag.ui.json_invalid'); return false; }
                    if (!data || typeof data !== 'object' || Array.isArray(data)) { err.hidden = false; err.textContent = t('easydrag.ui.test_data_object'); return false; }
                    const sel = d.body.querySelector('[data-ed-test-trigger]');
                    if (sel) trigger = ed.model.node(sel.value) || trigger;
                    const skip = d.body.querySelector('[data-ed-effects-skip]');
                    if (needConfirm) { ed.effectsConfirmed = true; if (skip && skip.checked) core.storage.set(EFFECTS_KEY + ed.flow.id, true); }
                    core.storage.set('aurago.easydrag.test-trigger.' + ed.flow.id, trigger.id);
                    return run(trigger.id, data, o.onlyNode, d.body.querySelector('[data-ed-test-remember]').checked).then(ok => ok ? true : false);
                }
            });
            return dialog;
        }

        async function run(triggerNode, data, onlyNode, remember) {
            try {
                const body = { trigger_node: triggerNode, only_node: onlyNode || undefined, remember_data: !!remember };
                if (data) body.trigger_data = data;
                const res = await ed.api.test(ed.flow.id, body);
                if (res.run_id) attach(res.run_id, { mode: 'test' });
                return true;
            } catch (err) {
                ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: core.errorText(t, err), type: 'error' });
                return false;
            }
        }

        async function runLive() {
            try {
                const res = await ed.api.runNow(ed.flow.id);
                if (res.run_id) attach(res.run_id, { mode: 'live' });
            } catch (err) {
                ed.ctx.notify({ title: t('easydrag.ui.run_now'), message: core.errorText(t, err), type: 'error' });
            }
        }

        async function cancel() {
            if (!ed.run || isFinal(ed.run.status)) return;
            try { await ed.api.cancel(ed.run.id); } catch (err) { ed.ctx.notify({ title: t('easydrag.ui.run_cancel'), message: core.errorText(t, err), type: 'error' }); }
        }

        // loadLast fills "Last run" data for the input tree when the editor opens.
        async function loadLast() {
            try {
                const runs = (await ed.api.runs(ed.flow.id, { limit: 1 })).runs || [];
                if (!runs.length) return;
                const detail = await ed.api.run(runs[0].id, false);
                rememberRun(detail.run, detail.steps || []);
                ed.bus.emit('last-run', detail.run);
            } catch (err) { /* no history yet */ }
        }

        // ── drawer ──────────────────────────────────────────────────────────────

        function triggerLabel(r) {
            const n = ed.model.node(r.trigger_node);
            return n ? (n.label || n.type) : t('easydrag.ui.trigger_removed');
        }

        async function loadRuns() {
            if (!drawer) return;
            const list = drawer.querySelector('.ed-runs-list');
            const params = { limit: 50 };
            if (drawerFilter === 'errors') params.status = 'error';
            if (drawerFilter === 'tests') params.mode = 'test';
            if (drawerFilter === 'live') params.mode = 'live';
            list.innerHTML = '<p class="ed-hint">' + esc(t('easydrag.ui.loading')) + '</p>';
            try {
                const runs = (await ed.api.runs(ed.flow.id, params)).runs || [];
                list.innerHTML = runs.length ? runs.map(r =>
                    '<button type="button" class="ed-run-row" data-ed-run="' + esc(r.id) + '"><span class="ed-run-dot ed-run-dot--' + esc(r.status) + '"></span>' +
                    '<span class="ed-run-main"><span>' + esc(core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(triggerLabel(r)) + '</span>' +
                    '<span class="ed-muted">' + esc(core.fmt.dateTime(r.started_at)) + (r.duration_ms ? ' · ' + esc(core.fmt.duration(r.duration_ms)) : '') + '</span></span>' +
                    '<span class="ed-chip' + (r.mode === 'test' ? ' ed-chip--muted' : '') + '">' + esc(r.mode === 'test' ? t('easydrag.ui.run_mode_test') : t('easydrag.ui.run_mode_live')) + '</span></button>').join('')
                    : '<p class="ed-hint">' + esc(t('easydrag.ui.runs_empty')) + '</p>';
            } catch (err) { list.innerHTML = '<p class="ed-error">' + esc(core.errorText(t, err)) + '</p>'; }
        }

        function toggleDrawer(force) {
            const open = force === undefined ? !drawer : force;
            if (!open) { if (drawer) { drawer.remove(); drawer = null; ed.root.classList.remove('has-drawer'); } return; }
            if (drawer) return;
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
                const row = event.target.closest('[data-ed-run]');
                if (row) openRunView(row.dataset.edRun);
            });
            loadRuns();
        }

        async function openRunView(runId) {
            try {
                const detail = await ed.api.run(runId, true);
                if (!detail.doc) { ed.ctx.notify({ title: t('easydrag.ui.runs_title'), message: t('easydrag.ui.run_view_missing'), type: 'error' }); return; }
                ed.enterRunView(detail);
            } catch (err) {
                ed.ctx.notify({ title: t('easydrag.ui.runs_title'), message: core.errorText(t, err), type: 'error' });
            }
        }

        bag.add(closeStream);
        bag.add(() => toggleDrawer(false));

        return {
            startTest, attach, runLive, cancel, loadLast, toggleDrawer, openRunView, effects,
            stepsFrom: (steps, record) => { const map = new Map(); (steps || []).forEach(s => map.set(s.node_id, s)); return { id: record.id, status: record.status, mode: record.mode, steps: map, record }; },
            applyRunView(detail) { setRun(this.stepsFrom(detail.steps, detail.run)); },
            clearRun() { closeStream(); setRun(null); },
            isRunning: () => !!(ed.run && !isFinal(ed.run.status)),
            drawerOpen: () => !!drawer,
            dispose() { bag.dispose(); }
        };
    }

    ED.runs = { create, effects, triggers, edgeStates };
})();
