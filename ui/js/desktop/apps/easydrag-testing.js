// Explicit simulation controls and bounded, text-only run results in the existing workspace.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const MAX_SAMPLE = 2 * 1024 * 1024;

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

    // runningSteps lists the steps a test runs, in document order. A test of the whole flow counts
    // every enabled step. A step test (onlyNode) runs what the engine runs
    // (internal/flows/engine_state.go, run, fireTrigger and collectReady): the step and its
    // ancestors, and of those only the steps the test's trigger reaches. Other triggers are
    // skipped, and a disabled step is skipped with what only it feeds. Every wire counts, whatever
    // its port, so the walk deliberately over-estimates what a branch will run. triggerId is the
    // test's trigger; without one, every enabled trigger counts.
    function runningSteps(ed, onlyNode, triggerId) {
        const doc = ed.model.doc;
        if (!onlyNode) return doc.nodes.filter(n => !n.settings.disabled);
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
        return doc.nodes.filter(n => runs.has(n.id));
    }

    // effects lists the outward effects of the steps a test runs (for the real-effects warning).
    // Each step counts with its catalog effects, which describe its default settings, and with
    // the effects of its real settings when the server named them (real: node id -> effects,
    // from realEffects): an HTTP request with POST sends, while its catalog entry (GET) does not.
    function effects(ed, onlyNode, triggerId, real) {
        const out = new Map();
        runningSteps(ed, onlyNode, triggerId).forEach(n => {
            const i = ed.model.info(n.type);
            const kinds = new Set((i && i.effects) || []);
            ((real && real.get(n.id)) || []).forEach(effect => kinds.add(effect));
            kinds.forEach(effect => {
                if (!out.has(effect)) out.set(effect, []);
                out.get(effect).push(n.label || n.type);
            });
        });
        return out;
    }

    // unknownEffects reports a step that runs and whose effects nobody could name: its catalog
    // entry is risky without effects, which is how the catalog marks an effects hook that failed
    // (internal/flows/catalog.go); CollectEffects leaves such a step out as well.
    function unknownEffects(ed, onlyNode, triggerId) {
        return runningSteps(ed, onlyNode, triggerId).some(n => {
            const i = ed.model.info(n.type);
            return !!(i && i.risky && !((i.effects || []).length));
        });
    }


    function parseSample(text) {
        if (typeof text !== 'string' || text.length > MAX_SAMPLE) throw new Error('sample');
        const value = JSON.parse(text || '{}');
        if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('sample');
        const stack = [value];
        let budget = 20000;
        while (stack.length) {
            const item = stack.pop();
            if (--budget < 0) throw new Error('sample');
            if (item === '[redacted]') throw new Error('sample');
            if (!item || typeof item !== 'object') continue;
            const keys = Object.keys(item);
            if (keys.includes('[redacted]') || typeof item._preview === 'string' && (keys.length === 1 || keys.length === 2 && (item._preview === '' && item._omitted === 'run_storage_budget' || item._truncated === true))) throw new Error('sample');
            Object.values(item).forEach(child => stack.push(child));
        }
        return value;
    }

    function decorate(ed, dialog, env) {
        const core = ED.core, { t, esc } = ed;
        let busy = false, closed = false;
        const controller = new AbortController();
        const mode = core.el('<label class="ed-label">' + esc(t('easydrag.ui.test_title')) + '<select class="ed-input" data-ed-test-mode><option value="real">' + esc(t('easydrag.ui.real_test')) + '</option><option value="simulation">' + esc(t('easydrag.ui.simulation_title')) + '</option></select></label>');
        const panel = core.el('<section class="ed-simulation-fields" hidden><p class="ed-callout">' + esc(t('easydrag.ui.simulation_hint')) + '</p><details class="ed-details" data-ed-fixtures-advanced><summary>' + esc(t('easydrag.ui.simulation_fixtures')) + '</summary><label class="ed-label">' + esc(t('easydrag.ui.simulation_fixtures')) + '<textarea class="ed-input ed-code" spellcheck="false" rows="7" data-ed-simulation-fixtures>{}</textarea></label><p class="ed-hint">' + esc(t('easydrag.ui.simulation_fixture_hint')) + '</p><ul class="ed-simulation-nodes">' + ed.model.doc.nodes.filter(n => !(ed.model.info(n.type) || {}).trigger).map(n => '<li><strong>' + esc(n.label || n.type) + '</strong> <code>' + esc(n.id) + '</code></li>').join('') + '</ul></details></section>');
        panel.querySelector('textarea').value = '{}';
        dialog.body.insertBefore(mode, dialog.body.firstChild);
        if (ED.durable && ED.durable.requires(ed.model.doc)) dialog.body.insertBefore(core.el('<p class="ed-hint">' + esc(t('easydrag.ui.durable_live_only')) + '</p>'), mode);
        dialog.body.appendChild(panel);
        const select = mode.querySelector('select');
        let scenarios = null;
        function syncMode() {
            const simulated = select.value === 'simulation';
            panel.hidden = !simulated;
            dialog.body.querySelectorAll('[data-ed-test-effects], [data-ed-effects-unchecked]').forEach(node => { node.hidden = simulated; });
            const remember = dialog.body.querySelector('[data-ed-test-remember]');
            if (remember) remember.disabled = simulated;
            const button = dialog.el.querySelector('[data-ed-action="run"] span');
            if (button) button.textContent = t(simulated ? 'easydrag.ui.simulation_run' : 'easydrag.ui.test_run');
            if (simulated && scenarios) scenarios.activate();
        }
        if (ED.scenarios && ed.api.scenarios) scenarios = ED.scenarios.create(ed, dialog, Object.assign({}, env, { panel, mode: select, fixtures: panel.querySelector('textarea'), syncMode }));
        select.addEventListener('change', syncMode);
        dialog.done.then(() => { closed = true; controller.abort(); });
        return {
            isSelected: () => select.value === 'simulation',
            syncMode,
            flush: () => !scenarios || scenarios.flush(),
            refreshData: () => { if (scenarios) scenarios.refreshData(); },
            async run() {
                if (busy || closed || !env.isCurrent()) return false;
                const error = dialog.body.querySelector('.ed-error');
                let data, fixtures, request;
                try {
                    request = scenarios && scenarios.request();
                    if (request === false) return false;
                    if (request) data = JSON.parse(dialog.body.querySelector('[data-ed-test-data]').value);
                    else {
                        data = parseSample(dialog.body.querySelector('[data-ed-test-data]').value);
                        fixtures = parseSample(panel.querySelector('textarea').value);
                    }
                } catch (err) { error.hidden = false; error.textContent = t('easydrag.ui.simulation_invalid'); return false; }
                busy = true;
                const revision = env.revision();
                try {
                    const result = await ed.api.simulate(ed.flow.id, request || { trigger_node: env.trigger(), trigger_data: data, only_node: env.onlyNode || undefined, base_revision: revision, fixtures }, controller.signal);
                    if (closed) return false;
                    if (!env.isCurrent() || Number(result.revision) !== revision) { error.hidden = false; error.textContent = t('easydrag.ui.error_flow_revision_conflict'); return false; }
                    if (result.error_code === 'FLOW_SIMULATION_FIXTURE_REQUIRED') {
                        panel.querySelector('[data-ed-fixtures-advanced]').open = true;
                        const node = ed.model.node(result.error_node_id);
                        error.hidden = false; error.textContent = core.errorText(t, { body: { code: result.error_code } }) + (node ? ' · ' + (node.label || node.type) : '');
                        return false;
                    }
                    env.apply(result, data);
                    return true;
                } catch (err) {
                    if (!closed) { error.hidden = false; error.textContent = core.errorText(t, err); }
                    return false;
                } finally { busy = false; }
            }
        };
    }

    function contentResult(ed, steps) {
        for (let i = steps.length - 1; i >= 0; i--) {
            const step = steps[i], node = ed.model.node(step.node_id);
            if (step.status !== 'success' || !node) continue;
            const fields = ed.model.fieldsOf(node);
            const text = ED.detail.previewText(step.output, fields);
            if (text) return { text, label: node.label || node.type };
        }
        return null;
    }

    function createResults(ed, canvas) {
        const core = ED.core, bag = core.bag(), { t, esc } = ed;
        const el = core.el('<details class="ed-run-results" hidden><summary></summary><div class="ed-run-results-body"></div></details>');
        canvas.el.appendChild(el);
        let signature = '', paintedRun = null, durableRunId = '', durablePhaseKnown = false, durableTouched = false;
        function render() {
            const run = ed.run;
            if (!run) { el.hidden = true; return; }
            const durable = ED.durable && ED.durable.has(run);
            const phase = durable ? JSON.stringify(ED.durable.info(run)) + '|' + !!run.durableUnknown + '|' + !!run.durableStopping : '';
            if (!ED.runs.isFinal(run.status)) {
                if (!durable) { el.hidden = true; return; }
                const next = run.id + '|' + run.status + '|' + phase;
                if (next === signature && run === paintedRun) return;
                signature = next; paintedRun = run; el.hidden = false;
                if (durableRunId !== run.id) { durableRunId = run.id; durablePhaseKnown = false; durableTouched = false; }
                const status = ED.durable.info(run);
                if (!durableTouched && !durablePhaseKnown) el.open = status?.state === 'blocked';
                if (status) durablePhaseKnown = true;
                el.querySelector('summary').textContent = t('easydrag.ui.durable_title') + ' · ' + ED.durable.label(t, run);
                el.querySelector('.ed-run-results-body').innerHTML = ED.durable.markup(ed, run);
                return;
            }
            const steps = Array.from(run.steps.values()).sort((a, b) => String(a.started_at || '').localeCompare(String(b.started_at || '')));
            const key = [run.id, run.mode, run.status, run.record && run.record.finished_at, phase, steps.map(step => step.node_id + ':' + step.status).join(',')].join('|');
            if (key === signature && run === paintedRun) return;
            signature = key;
            paintedRun = run;
            el.hidden = false;
            const simulation = run.mode === 'simulation';
            el.open = simulation || run.status === 'error';
            el.querySelector('summary').textContent = (simulation ? t('easydrag.ui.simulation_result') : t('easydrag.ui.run_diagnosis')) + ' · ' + core.tr(t, 'easydrag.ui.status_' + run.status, run.status);
            const failed = steps.find(step => step.status === 'error');
            const complete = steps.filter(step => step.status === 'success');
            const preview = ed.isBlock && run.outputs ? null : contentResult(ed, complete);
            const label = step => { const n = ed.model.node(step.node_id); return n ? n.label || (ed.model.info(n.type) || {}).label || n.type : step.node_key || step.node_id; };
            el.querySelector('.ed-run-results-body').innerHTML =
                (durable ? ED.durable.markup(ed, run) : '') +
                (simulation ? '<p class="ed-hint">' + esc(t(ed.isBlock ? 'easydrag.ui.block_simulation_hint' : 'easydrag.ui.simulation_hint')) + '</p>' : '') +
                (simulation && ed.isBlock && run.status === 'success' ? ED.blocks.outputMarkup(ed, run.outputs) : '') +
                (ED.batches ? complete.filter(step => ed.model.node(step.node_id)?.type === 'component.each').map(step => ED.batches.resultMarkup(ed, step, true)).join('') : '') +
                (failed ? '<section class="ed-run-diagnosis"><h3>' + esc(label(failed)) + '</h3><p>' + esc(core.stepErrorText(t, failed)) + '</p><p>' + esc(t('easydrag.ui.run_next_action')) + '</p>' + (!simulation ? '<h4>' + esc(t('easydrag.ui.run_completed_effects')) + '</h4><p>' + esc(complete.map(label).join(' · ') || '—') + '</p>' : '') + '</section>' : '') +
                (preview ? '<section class="ed-result-preview"><h4>' + esc(t('easydrag.ui.output_preview') + ' · ' + preview.label) + '</h4><p>' + esc(preview.text.slice(0, 1500)) + '</p></section>' : '') +
                (simulation && run.record && run.record.assertions && run.record.assertions.length ? '<h4>' + esc(t('easydrag.ui.scenario_assertions')) + '</h4><ul class="ed-assertion-results">' + run.record.assertions.map(assertion => '<li><strong>' + esc(label(assertion)) + ' · ' + esc(t(assertion.passed ? 'easydrag.ui.scenario_assertion_passed' : 'easydrag.ui.scenario_assertion_failed')) + '</strong>' + (assertion.code ? '<p>' + esc(core.tr(t, 'easydrag.ui.assertion_' + assertion.code, assertion.code)) + '</p>' : '') + '</li>').join('') + '</ul>' : '') +
                '<h4>' + esc(t('easydrag.ui.run_timeline')) + '</h4><ol class="ed-run-timeline">' + steps.map(step => '<li><button type="button" data-ed-result-node="' + esc(step.node_id) + '"><span class="ed-run-dot ed-run-dot--' + esc(step.status) + '"></span><span>' + esc(label(step)) + '</span><span class="ed-muted">' + esc(core.tr(t, 'easydrag.ui.status_' + step.status, step.status)) + (step.duration_ms ? ' · ' + esc(core.fmt.duration(step.duration_ms)) : '') + (step.effect_uncertain ? ' · ' + esc(t('easydrag.ui.step_effect_uncertain')) : '') + '</span></button></li>').join('') + '</ol>';
        }
        bag.listen(el, 'pointerdown', event => event.stopPropagation());
        bag.listen(el, 'wheel', event => event.stopPropagation());
        bag.listen(el, 'click', event => {
            if (event.target.closest('summary')) durableTouched = true;
            if (event.target.closest('[data-ed-durable-check]')) { ed.bus.emit('refresh-durable-run'); return; }
            const button = event.target.closest('[data-ed-result-node]');
            if (!button) return;
            ed.bus.emit('focus-node', button.dataset.edResultNode);
            ed.bus.emit('open-detail', { nodeId: button.dataset.edResultNode, column: 'output' });
        });
        bag.add(ed.bus.on('run', render));
        return { dispose() { bag.dispose(); el.remove(); } };
    }
    ED.testing = { decorate, createResults, parseSample, contentResult, triggers, realEffects, effects, unknownEffects };
})();
