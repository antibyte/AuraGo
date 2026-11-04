// Private simulation cases never round-trip scrubbed snapshots as authoritative input.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};

    function readAssertions(rows) {
        return rows.map(row => {
            const assertion = { node_id: row.node_id }, output = row.output.trim(), ports = row.ports.trim();
            if (!assertion.node_id || (!output && !ports)) throw new Error('assertion');
            if (output) assertion.output = JSON.parse(output);
            if (ports) assertion.ports = ports.split(',').map(port => port.trim()).filter(Boolean);
            return assertion;
        });
    }

    function create(ed, dialog, env) {
        const core = ED.core, { t, esc } = ed;
        const area = dialog.body.querySelector('[data-ed-test-data]');
        const originalLabel = area.parentNode;
        const fields = core.el('<section class="ed-sample-fields"><h4>' + esc(t('easydrag.ui.sample_fields')) + '</h4><p class="ed-hint">' + esc(t('easydrag.ui.sample_fields_hint')) + '</p><div data-ed-sample-fields></div></section>');
        const advanced = core.el('<details class="ed-details"><summary>' + esc(t('easydrag.ui.sample_json')) + '</summary></details>');
        originalLabel.parentNode.insertBefore(fields, originalLabel);
        originalLabel.parentNode.insertBefore(advanced, originalLabel);
        advanced.appendChild(originalLabel);
        const section = core.el('<section class="ed-scenario-fields"><h4>' + esc(t('easydrag.ui.scenario_title')) + '</h4><p class="ed-hint">' + esc(t('easydrag.ui.scenario_hint')) + '</p>' +
            '<label class="ed-label">' + esc(t('easydrag.ui.scenario_title')) + '<select class="ed-input" data-ed-scenario><option value="">' + esc(t('easydrag.ui.scenario_current')) + '</option></select></label>' +
            '<p class="ed-callout" data-ed-scenario-state hidden></p><div class="ed-scenario-actions"><button type="button" class="ed-btn" data-ed-scenario-new>' + esc(t('easydrag.ui.scenario_new')) + '</button><button type="button" class="ed-btn" data-ed-scenario-rename hidden>' + esc(t('easydrag.ui.scenario_rename')) + '</button><button type="button" class="ed-btn" data-ed-scenario-delete hidden>' + esc(t('easydrag.ui.scenario_delete')) + '</button></div>' +
            '<div data-ed-scenario-save><label class="ed-label">' + esc(t('easydrag.ui.scenario_name')) + '<input class="ed-input" data-ed-scenario-name></label><button type="button" class="ed-btn" data-ed-scenario-save-button>' + esc(t('easydrag.ui.scenario_save')) + '</button></div>' +
            '<section class="ed-assertions"><h4>' + esc(t('easydrag.ui.scenario_assertions')) + '</h4><p class="ed-hint">' + esc(t('easydrag.ui.scenario_assertions_hint')) + '</p><div data-ed-assertions></div><button type="button" class="ed-btn" data-ed-assertion-add>' + core.icon('plus') + esc(t('easydrag.ui.scenario_assertion_add')) + '</button></section></section>');
        env.panel.appendChild(section);
        const picker = section.querySelector('[data-ed-scenario]'), state = section.querySelector('[data-ed-scenario-state]');
        const controller = new AbortController();
        let selected = null, closed = false, loaded = false, busy = false, choosing = false, selection = 0, sampleSignature = '';
        const invalidFields = new Set();
        const error = dialog.body.querySelector('.ed-error');
        const alive = () => !closed && env.isCurrent();
        function report(err) { if (!closed) { error.hidden = false; error.textContent = core.errorText(t, err); } }
        function lock() {
            const stored = !!selected;
            area.readOnly = stored;
            env.fixtures.readOnly = stored;
            env.mode.disabled = stored;
            const trigger = dialog.body.querySelector('[data-ed-test-trigger]');
            if (trigger) trigger.disabled = stored;
            section.querySelector('[data-ed-scenario-save]').hidden = stored;
            section.querySelector('[data-ed-scenario-rename]').hidden = !stored;
            section.querySelector('[data-ed-scenario-delete]').hidden = !stored;
            section.querySelector('[data-ed-assertion-add]').hidden = stored;
            section.querySelectorAll('.ed-assertion-row input, .ed-assertion-row textarea, .ed-assertion-row select, .ed-assertion-row button').forEach(input => { input.disabled = stored; });
            state.hidden = !stored;
            state.textContent = stored ? t(selected.stale ? 'easydrag.ui.scenario_stale' : 'easydrag.ui.scenario_protected') : '';
            refreshData();
            fields.querySelectorAll('input, select').forEach(input => { input.disabled = stored; });
        }
        function refreshData(force) {
            if (!force && sampleSignature === area.value) return;
            sampleSignature = area.value;
            invalidFields.clear();
            const host = fields.querySelector('[data-ed-sample-fields]');
            let value;
            try { value = ED.testing.parseSample(area.value); } catch (_) { host.innerHTML = ''; advanced.open = true; return; }
            const keys = Object.keys(value).filter(key => ['string', 'number', 'boolean'].includes(typeof value[key])).slice(0, 100);
            host.innerHTML = keys.map((key, index) => '<label class="ed-label">' + esc(key) + (typeof value[key] === 'boolean' ? '<select class="ed-input" data-ed-sample-field="' + index + '"><option value="true"' + (value[key] ? ' selected' : '') + '>true</option><option value="false"' + (!value[key] ? ' selected' : '') + '>false</option></select>' : '<input class="ed-input"' + (typeof value[key] === 'number' ? ' type="number" step="any"' : '') + ' data-ed-sample-field="' + index + '" value="' + esc(value[key]) + '">') + '</label>').join('');
            if (!keys.length) advanced.open = true;
            host.querySelectorAll('[data-ed-sample-field]').forEach(input => {
                input.disabled = !!selected;
                const key = keys[Number(input.dataset.edSampleField)];
                const change = () => {
                    const next = typeof value[key] === 'number' ? Number(input.value) : typeof value[key] === 'boolean' ? input.value === 'true' : input.value;
                    if (typeof value[key] === 'number' && (!input.value.trim() || !Number.isFinite(next))) { invalidFields.add(input); input.setCustomValidity && input.setCustomValidity(t('easydrag.ui.json_invalid')); return; }
                    invalidFields.delete(input);
                    input.setCustomValidity && input.setCustomValidity('');
                    // Re-read JSON so nested and advanced-only fields are retained.
                    try { const all = ED.testing.parseSample(area.value); all[key] = next; area.value = JSON.stringify(all, null, 2); sampleSignature = area.value; } catch (_) { advanced.open = true; }
                };
                input.addEventListener('input', change); input.addEventListener('change', change);
            });
        }
        function assertionRow(assertion) {
            const nodeId = assertion && assertion.node_id || '';
            const row = core.el('<div class="ed-assertion-row"><label class="ed-label">' + esc(t('easydrag.ui.scenario_assertion_step')) + '<select class="ed-input" data-ed-assertion-node>' + ed.model.doc.nodes.map(node => '<option value="' + esc(node.id) + '"' + (node.id === nodeId ? ' selected' : '') + '>' + esc(node.label || node.type) + '</option>').join('') + '</select></label><label class="ed-label">' + esc(t('easydrag.ui.scenario_assertion_output')) + '<textarea class="ed-input ed-code" rows="3" data-ed-assertion-output>' + esc(assertion && Object.prototype.hasOwnProperty.call(assertion, 'output') ? JSON.stringify(assertion.output, null, 2) : '') + '</textarea></label><label class="ed-label">' + esc(t('easydrag.ui.scenario_assertion_ports')) + '<input class="ed-input" data-ed-assertion-ports value="' + esc(assertion && assertion.ports ? assertion.ports.join(', ') : '') + '"></label><button type="button" class="ed-btn" data-ed-assertion-remove>' + esc(t('easydrag.ui.delete')) + '</button></div>');
            if (selected) row.querySelectorAll('input, textarea, select, button').forEach(input => { input.disabled = true; });
            row.querySelector('[data-ed-assertion-remove]').addEventListener('click', () => row.remove());
            section.querySelector('[data-ed-assertions]').appendChild(row);
        }
        function assertions() {
            return readAssertions(Array.from(section.querySelectorAll('.ed-assertion-row')).map(row => ({ node_id: row.querySelector('[data-ed-assertion-node]').value, output: row.querySelector('[data-ed-assertion-output]').value, ports: row.querySelector('[data-ed-assertion-ports]').value })));
        }
        async function list() {
            const result = await ed.api.scenarios(ed.flow.id, controller.signal);
            if (!alive()) return;
            picker.innerHTML = '<option value="">' + esc(t('easydrag.ui.scenario_current')) + '</option>' + (result.scenarios || []).map(item => '<option value="' + esc(item.id) + '">' + esc(item.name) + (item.stale ? ' · ' + esc(t('easydrag.ui.state_changes')) : '') + '</option>').join('');
            picker.value = selected ? selected.id : '';
        }
        async function choose(id) {
            const seq = ++selection;
            if (!id) { choosing = false; selected = null; lock(); return; }
            choosing = true;
            try {
                const result = await ed.api.scenario(ed.flow.id, id, controller.signal);
                if (!alive() || seq !== selection) return;
                selected = result.scenario;
                const trigger = dialog.body.querySelector('[data-ed-test-trigger]');
                if (trigger) trigger.value = selected.trigger_node;
                area.value = JSON.stringify(selected.trigger_data || {}, null, 2);
                env.fixtures.value = JSON.stringify(selected.fixtures || {}, null, 2);
                env.mode.value = 'simulation';
                section.querySelector('[data-ed-assertions]').innerHTML = '';
                (selected.assertions || []).forEach(assertionRow);
                lock(); env.syncMode();
            } catch (err) { picker.value = selected ? selected.id : ''; report(err); }
            finally { if (seq === selection) choosing = false; }
        }
        picker.addEventListener('change', () => choose(picker.value));
        section.querySelector('[data-ed-scenario-new]').addEventListener('click', () => {
            selection++; choosing = false; selected = null; picker.value = ''; area.value = '{}'; env.fixtures.value = '{}';
            section.querySelector('[data-ed-assertions]').innerHTML = ''; section.querySelector('[data-ed-scenario-name]').value = '';
            lock(); env.syncMode();
        });
        section.querySelector('[data-ed-assertion-add]').addEventListener('click', () => { if (!selected && section.querySelectorAll('.ed-assertion-row').length < 100) assertionRow(); });
        async function mutate(action) {
            if (busy || choosing || closed) return;
            if (!env.isCurrent()) { error.hidden = false; error.textContent = t('easydrag.ui.error_flow_revision_conflict'); return; }
            busy = true;
            picker.disabled = true;
            section.querySelectorAll('button').forEach(button => { button.disabled = true; });
            try { await action(); } catch (err) { report(err); }
            finally { busy = false; if (!closed) { picker.disabled = false; section.querySelectorAll('button').forEach(button => { button.disabled = false; }); lock(); } }
        }
        function validName(name) { if (!name || Array.from(name).length > 80) { error.hidden = false; error.textContent = t('easydrag.ui.scenario_name_invalid'); return false; } return true; }
        section.querySelector('[data-ed-scenario-save-button]').addEventListener('click', () => mutate(async () => {
            if (selected) return;
            const name = section.querySelector('[data-ed-scenario-name]').value.trim();
            if (!validName(name) || !flush(true)) return;
            let expected;
            try { expected = assertions(); } catch (_) { error.hidden = false; error.textContent = t('easydrag.ui.scenario_assertion_invalid'); return; }
            let data, fixtures;
            try { data = ED.testing.parseSample(area.value); fixtures = ED.testing.parseSample(env.fixtures.value); }
            catch (_) { error.hidden = false; error.textContent = t('easydrag.ui.error_flow_scenario_invalid'); return; }
            const scenario = { name, trigger_node: env.trigger(), trigger_data: data, fixtures, assertions: expected };
            if (env.onlyNode) scenario.only_node = env.onlyNode;
            const result = await ed.api.saveScenario(ed.flow.id, { base_revision: env.revision(), expected_version: 0, scenario });
            if (!alive()) return;
            selected = result.scenario; await list(); await choose(selected.id);
        }));
        section.querySelector('[data-ed-scenario-rename]').addEventListener('click', () => mutate(async () => {
            const previous = selected;
            if (!previous) return;
            const input = await ed.ctx.promptDialog(t('easydrag.ui.scenario_rename'), previous.name);
            if (input == null || !alive() || selected !== previous) return;
            const name = input.trim(); if (!validName(name)) return;
            const result = await ed.api.renameScenario(ed.flow.id, previous.id, name, previous.version);
            if (!alive()) return;
            selected = Object.assign({}, previous, result.scenario); await list();
        }));
        section.querySelector('[data-ed-scenario-delete]').addEventListener('click', () => mutate(async () => {
            const previous = selected;
            if (!previous || !(await ed.ctx.confirmDialog(t('easydrag.ui.scenario_delete'), t('easydrag.ui.scenario_delete_confirm', { name: previous.name }))) || !alive() || selected !== previous) return;
            await ed.api.deleteScenario(ed.flow.id, previous.id, previous.version);
            if (!alive()) return;
            selected = null; area.value = '{}'; env.fixtures.value = '{}'; section.querySelector('[data-ed-assertions]').innerHTML = ''; await list();
        }));
        area.addEventListener('input', () => refreshData());
        function flush(inMutation) {
            if (choosing || (busy && !inMutation)) return false;
            const invalid = Array.from(fields.querySelectorAll('input')).find(input => invalidFields.has(input) || input.validity && !input.validity.valid);
            if (invalid) { if (invalid.reportValidity) invalid.reportValidity(); return false; }
            return true;
        }
        dialog.done.then(() => { closed = true; selection++; controller.abort(); selected = null; });
        refreshData();
        return {
            refreshData, flush, selected: () => selected,
            activate() { if (!loaded) { loaded = true; list().catch(err => { loaded = false; report(err); }); } },
            request() {
                if (choosing || busy) return false;
                if (!selected) return null;
                if (selected.stale) { error.hidden = false; error.textContent = t('easydrag.ui.scenario_stale'); return false; }
                return { base_revision: env.revision(), scenario_id: selected.id, scenario_version: selected.version };
            }
        };
    }
    ED.scenarios = { create, readAssertions };
})();
