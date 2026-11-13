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

    // Service.Simulate runs on a registry without the generic tool types and refuses a durable
    // step (internal/flows/simulation.go), and it validates every draft node, disabled and
    // unreachable ones included. The catalog says per type whether it can simulate it
    // (simulatable). An older server sends no flag: the durable types and every tool.* type
    // count as not simulatable, any other type as unknown, which keeps the old behaviour.
    const DURABLE_TYPES = new Set(['logic.durable_wait', 'logic.durable_approval']);
    let modeIds = 0;

    function canSimulate(ed, node) {
        const i = ed.model.info(node.type);
        if (i && i.simulatable === false) return false;
        return !(DURABLE_TYPES.has(node.type) || String(node.type).startsWith('tool.'));
    }

    // simulationDefault: a test whose steps have outward effects, named (catalog or publish
    // preview, see effects) or unknown (a risky step without named effects), opens in
    // Simulation. A flow without effects, or with a step the simulator cannot run, opens as a
    // real test.
    function simulationDefault(ed, onlyNode, triggerId, real) {
        if (!ed.model.doc.nodes.every(n => canSimulate(ed, n))) return false;
        return effects(ed, onlyNode, triggerId, real).size > 0 || unknownEffects(ed, onlyNode, triggerId);
    }

    const UNSAFE_PATH = new Set(['__proto__', 'constructor', 'prototype']);

    // localSample shows a built-in trigger sample in the UI language. The catalog names the
    // text fields of each sample (sample_text_keys: event variant -> dotted path -> key; ""
    // is the default sample, as in withEventSamples on the server), and those fields take
    // t(key). A saved sample, or an answer without a source, is left as it is.
    function localSample(ed, node, answer) {
        const data = answer && answer.data && typeof answer.data === 'object' && !Array.isArray(answer.data) ? answer.data : {};
        if (!answer || answer.source !== 'builtin' || !node) return data;
        const byEvent = (ed.model.info(node.type) || {}).sample_text_keys;
        if (!byEvent || typeof byEvent !== 'object') return data;
        const event = String((node.params || {}).event == null ? '' : node.params.event).trim();
        const keys = event && Object.prototype.hasOwnProperty.call(byEvent, event) ? byEvent[event] : byEvent[''];
        if (!keys || typeof keys !== 'object') return data;
        Object.keys(keys).forEach(path => {
            const parts = path.split('.');
            if (parts.some(part => UNSAFE_PATH.has(part)) || typeof keys[path] !== 'string') return;
            let holder = data;
            for (let i = 0; i < parts.length - 1 && holder; i++) holder = typeof holder === 'object' ? holder[parts[i]] : null;
            const last = parts[parts.length - 1];
            if (!holder || typeof holder !== 'object' || typeof holder[last] !== 'string') return;
            const text = ed.t(keys[path]);
            if (typeof text === 'string' && text && text !== keys[path]) holder[last] = text;
        });
        return data;
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
        // The select is named by its own label only (an embedded select would add its option text).
        const modeLabel = 'ed-test-mode-' + (++modeIds);
        const mode = core.el('<label class="ed-label"><span id="' + modeLabel + '">' + esc(t('easydrag.ui.test_mode')) + '</span><select class="ed-input" data-ed-test-mode aria-labelledby="' + modeLabel + '"><option value="real">' + esc(t('easydrag.ui.real_test')) + '</option><option value="simulation">' + esc(t('easydrag.ui.simulation_title')) + '</option></select></label>');
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
        // The preselect dispatches no event, so a change is the user's own choice.
        let modeChosen = false;
        select.addEventListener('change', () => { modeChosen = true; syncMode(); });
        // A test of steps with outward effects opens in Simulation (UX-07). The scope is the
        // one openTest checks: the tested step (env.onlyNode) and the chosen trigger.
        // preselect applies the default again when the trigger or the checked effects change,
        // until the user picks a mode; it never moves a chosen or simulated mode back to real.
        function preselect(triggerId, real) {
            if (modeChosen || select.disabled || select.value === 'simulation') return;
            if (simulationDefault(ed, env.onlyNode, triggerId, real)) { select.value = 'simulation'; syncMode(); }
        }
        preselect(typeof env.trigger === 'function' ? env.trigger() : undefined, typeof env.real === 'function' ? env.real() : null);
        dialog.done.then(() => { closed = true; controller.abort(); });
        return {
            isSelected: () => select.value === 'simulation',
            syncMode,
            preselect,
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
            el.querySelector('summary').textContent = (simulation ? t('easydrag.ui.simulation_result') : t('easydrag.ui.run_diagnosis')) + ' · ' + statusLabel(t, ed, run);
            const failed = steps.find(step => step.status === 'error');
            const stopped = stoppedBy({ run, steps }, ed.model); el.querySelector('summary').title = stopped ? t('easydrag.ui.run_stopped_by', { step: stopped.label }) : '';
            const complete = steps.filter(step => step.status === 'success');
            const preview = ed.isBlock && run.outputs ? null : contentResult(ed, complete);
            const label = step => { const n = ed.model.node(step.node_id); return n ? n.label || (ed.model.info(n.type) || {}).label || n.type : step.node_key || step.node_id; };
            el.querySelector('.ed-run-results-body').innerHTML =
                (durable ? ED.durable.markup(ed, run) : '') +
                (simulation ? '<p class="ed-hint">' + esc(t(ed.isBlock ? 'easydrag.ui.block_simulation_hint' : 'easydrag.ui.simulation_hint')) + '</p>' : '') +
                (simulation && ed.isBlock && run.status === 'success' ? ED.blocks.outputMarkup(ed, run.outputs) : '') +
                (ED.batches ? complete.filter(step => ed.model.node(step.node_id)?.type === 'component.each').map(step => ED.batches.resultMarkup(ed, step, true)).join('') : '') +
                (failed ? '<section class="ed-run-diagnosis"><h3>' + esc(label(failed)) + '</h3><p>' + esc(core.stepErrorText(t, failed)) + '</p><p>' + esc(t('easydrag.ui.run_next_action')) + '</p>' + (!simulation ? '<h4>' + esc(t('easydrag.ui.run_completed_effects')) + '</h4><p>' + esc(complete.map(label).join(' · ') || '—') + '</p>' : '') + '</section>' : '') +
                (!failed && run.record && run.record.error_code ? '<p class="ed-hint">' + esc(core.stepErrorText(t, run.record)) + '</p>' : '') +
                (preview ? '<section class="ed-result-preview"><h4>' + esc(t('easydrag.ui.output_preview') + ' · ' + preview.label) + '</h4><p>' + esc(preview.text.slice(0, 1500)) + '</p></section>' : '') +
                (simulation && run.record && run.record.assertions && run.record.assertions.length ? '<h4>' + esc(t('easydrag.ui.scenario_assertions')) + '</h4><ul class="ed-assertion-results">' + run.record.assertions.map(assertion => '<li><strong>' + esc(label(assertion)) + ' · ' + esc(t(assertion.passed ? 'easydrag.ui.scenario_assertion_passed' : 'easydrag.ui.scenario_assertion_failed')) + '</strong>' + (assertion.code ? '<p>' + esc(core.tr(t, 'easydrag.ui.assertion_' + assertion.code, assertion.code)) + '</p>' : '') + '</li>').join('') + '</ul>' : '') +
                '<h4>' + esc(t('easydrag.ui.run_timeline')) + '</h4><ol class="ed-run-timeline">' + steps.map(step => '<li><button type="button" data-ed-result-node="' + esc(step.node_id) + '">' + ED.status.disc(ED.status.ofStep(step, simulation), 14, { t, hidden: true }) + '<span>' + esc(label(step)) + '</span><span class="ed-muted">' + esc(ED.status.word(t, ED.status.ofStep(step, simulation))) + (step.duration_ms ? ' · ' + esc(core.fmt.duration(step.duration_ms)) : '') + '</span></button></li>').join('') + '</ol>';
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
    // stoppedBy returns the logic.stop step that ended a successful run, or null (S0c A2). The
    // stop step is a success for both stop choices, so the run's own status decides: a stop with
    // status "error" and a failure inside the stop's grace end the run as an error and get no hint.
    // detail is { run: { status }, steps: [step], doc? } as GET runs/{id}?include=doc answers it.
    // The node comes from detail.doc; a run whose document is gone (a pruned version) takes the
    // model node with the same id, and a node that is nowhere known gives null. The result is
    // { step, node, label }; label is the node's own label, else its type's label, else the type.
    function stoppedBy(detail, model) {
        const run = detail && detail.run;
        if (!run || run.status !== 'success' || !Array.isArray(detail.steps)) return null;
        const docNodes = detail.doc && Array.isArray(detail.doc.nodes) ? detail.doc.nodes : null;
        for (const step of detail.steps) {
            if (!step || step.status !== 'success') continue;
            const node = docNodes ? docNodes.find(n => n && n.id === step.node_id) : (model && typeof model.node === 'function' ? model.node(step.node_id) : null);
            if (!node || node.type !== 'logic.stop') continue;
            const info = model && typeof model.info === 'function' ? model.info(node.type) : null;
            return { step, node, label: node.label || (info && info.label) || node.type };
        }
        return null;
    }
    // statusOf is the status value of a run (ED.status): the durable and plain run statuses, and "stopped" for a successful run a
    // logic.stop ended (S0c A2, W1a A). The status replaces S0c's separate hint text.
    function statusOf(ed, run) {
        const base = ED.durable ? ED.durable.value(run) : ED.status.of(run && (run.record || run).status, 'run');
        if (base !== 'ok' || !run) return base;
        const record = run.record || run;
        const steps = run.steps && typeof run.steps.values === 'function' ? Array.from(run.steps.values()) : [];
        return stoppedBy({ run: { status: record.status }, steps }, ed.model) ? 'stopped' : base;
    }
    // statusLabel is the word for statusOf; "stopping" and the durable wording stay with ED.durable.label.
    function statusLabel(t, ed, run) {
        return statusOf(ed, run) === 'stopped' ? ED.status.word(t, 'stopped') : ED.durable ? ED.durable.label(t, run) : ED.status.word(t, ED.status.of(run && (run.record || run).status, 'run'));
    }
    ED.testing = { decorate, createResults, parseSample, contentResult, triggers, realEffects, effects, unknownEffects, simulationDefault, localSample, stoppedBy, statusOf, statusLabel };
})();
