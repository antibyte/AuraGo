// Typed block boundaries and explicit, immutable call-version bindings.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key);
    const clone = value => JSON.parse(JSON.stringify(value));
    const empty = type => ({ string: '', number: 0, boolean: false, object: {}, array: [] })[type];
    function validFields(fields) {
        if (!Array.isArray(fields) || fields.length > 64 || fields.some(field => !field || typeof field !== 'object' || Array.isArray(field) || !/^[a-z][a-z0-9_]{0,39}$/.test(field.name) || !own(ED.blocks.types, field.type) || Object.keys(field).some(key => !['name', 'type', 'required', 'nullable'].includes(key)) || ['required', 'nullable'].some(key => own(field, key) && typeof field[key] !== 'boolean')) || new Set(fields.map(field => field.name)).size !== fields.length) return false;
        try { return ED.transform.boundedJSON(fields).length <= 32768; } catch (_) { return false; }
    }

    // Reuse ordinary template/number/boolean/JSON controls, including caret and IME handling.
    function values(options) {
        const { ed, fields } = options, core = ED.core, prefix = options.prefix || '';
        if (options.requireComplete) ED.transform.boundedJSON(options.value || {});
        let data = clone(options.value || {}), closed = false, inner, fieldBag = core.bag();
        const rejected = new Map();
        const rejectionText = kind => ed.t(kind === 'incomplete' ? 'easydrag.ui.test_data_incomplete' : 'easydrag.ui.block_sample_limit');
        const el = core.el('<div class="ed-block-values">' + (options.requireComplete ? '<details class="ed-details"><summary>' + ed.esc(ed.t('easydrag.ui.transform_notes')) + '</summary><p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_sample_limit')) + '</p></details>' : '') + '<div data-ed-block-value-form></div><div data-ed-block-unknown></div><p class="ed-error" role="alert" hidden></p></div>');
        const names = new Set(fields.map(field => field.name));
        function valid(field) {
            if (!own(data, field.name)) return !field.required;
            const value = data[field.name];
            if (value === null) return field.nullable;
            if (options.templatable && typeof value === 'string' && value.includes('{{')) return true;
            return field.type === 'array' ? Array.isArray(value) : field.type === 'object' ? typeof value === 'object' && !Array.isArray(value) : field.type === 'number' ? typeof value === 'number' && Number.isFinite(value) : typeof value === field.type;
        }
        function unknown() { return Object.keys(data).filter(name => !names.has(name)); }
        function showUnknown() {
            const keys = unknown(), host = el.querySelector('[data-ed-block-unknown]');
            const preview = value => { const raw = JSON.stringify(value, null, 2); return raw.length > 2048 ? raw.slice(0, 2047).replace(/[\uD800-\uDBFF]$/, '') + '…' : raw; };
            host.innerHTML = !keys.length ? '' : '<details class="ed-details" open><summary>' + ed.esc(ed.t('easydrag.ui.block_unknown_bindings')) + '</summary><ul>' + keys.map((name, index) => '<li><code>' + ed.esc(name) + '</code><pre class="ed-code ed-output-json">' + ed.esc(preview(data[name])) + '</pre><button type="button" class="ed-btn" data-ed-block-unknown-remove="' + index + '"' + (ed.readonly || ed.runView ? ' disabled' : '') + '>' + ed.esc(ed.t('easydrag.ui.delete')) + '</button></li>').join('') + '</ul></details>';
            host.querySelectorAll('[data-ed-block-unknown-remove]').forEach(button => fieldBag.listen(button, 'click', () => {
                if (closed || ed.readonly || ed.runView || !inner.flush()) return;
                delete data[keys[Number(button.dataset.edBlockUnknownRemove)]];
                if (options.onChange) options.onChange(clone(data));
                showUnknown();
            }));
        }
        function params() { return Object.fromEntries(fields.map(field => [prefix + field.name, data[field.name]]).filter(pair => own(data, pair[0].slice(prefix.length)))); }
        function synthetic() { return { id: options.nodeId || 'block_sample', key: 'block_values', type: 'block.bindings', params: params(), settings: {} }; }
        function candidate(name, value) {
            if (!options.requireComplete) return;
            const next = Object.assign(Object.create(null), data);
            if (value === undefined) delete next[name]; else next[name] = value;
            ED.transform.boundedJSON(next);
        }
        function readJSON(name, raw) {
            try {
                let value;
                try { value = ED.transform.readSample(raw); } catch (err) { if (err.transformKind !== 'no_sample') throw err; }
                candidate(name, value); rejected.delete(name); return value;
            } catch (err) { rejected.set(name, err.transformKind || 'invalid'); throw err; }
        }
        function changed(name, value) {
            if (closed || ed.readonly || ed.runView) return;
            const field = name.slice(prefix.length);
            if (!names.has(field)) return;
            try { candidate(field, value); rejected.delete(field); }
            catch (err) { rejected.set(field, err.transformKind || 'invalid'); const error = Array.from(el.children).find(node => node.classList.contains('ed-error')); error.hidden = false; error.textContent = rejectionText(err.transformKind); return; }
            if (value === undefined) delete data[field]; else data[field] = clone(value);
            if (options.onChange) options.onChange(clone(data), field);
            if (inner) inner.refresh(synthetic(), options.roots || {});
            syncPresence();
        }
        const info = { type: 'block.bindings', params: fields.map(field => ({ name: prefix + field.name, label: field.name, kind: field.type === 'object' || field.type === 'array' ? 'json' : ED.blocks.types[field.type], literal: !options.templatable && field.type === 'string', required: field.required, templatable: !!options.templatable, parseJSON: options.requireComplete ? raw => readJSON(field.name, raw) : undefined, help: ed.t('easydrag.ui.type_' + ED.blocks.types[field.type]) + (field.nullable ? ' · null' : '') })) };
        function buildInner() {
            fieldBag.dispose(); fieldBag = core.bag();
            if (inner) inner.dispose(); inner = null;
            inner = ED.forms.render({ ed, node: synthetic(), info, upstream: options.upstream || [], roots: options.roots || {}, issues: [], onChange: changed, onRendered: () => { if (inner) syncPresence(); } });
            el.querySelector('[data-ed-block-value-form]').appendChild(inner.el); syncPresence(); showUnknown();
        }
        function syncPresence() {
            for (const field of fields) {
                const host = Array.from(inner.el.querySelectorAll('.ed-field')).find(node => node.dataset.param === prefix + field.name);
                if (!host) continue;
                let select = host.querySelector('[data-ed-block-presence]');
                if (!select) {
                    select = core.el('<select class="ed-input ed-input--small" data-ed-block-presence aria-label="' + ed.esc(ed.t('easydrag.ui.block_value_state') + ' · ' + field.name) + '"><option value="absent">' + ed.esc(ed.t('easydrag.ui.block_value_absent')) + '</option><option value="value">' + ed.esc(ed.t('easydrag.ui.mode_fixed')) + '</option>' + (field.nullable || data[field.name] === null ? '<option value="null"' + (field.nullable ? '' : ' disabled') + '>null</option>' : '') + '</select>');
                    // The presence choice commits in its own handler; ordinary form flush must
                    // not redispatch change on the focused select and recursively enter flush.
                    select.edCommit = () => true;
                    host.querySelector('.ed-field-head').appendChild(select);
                    fieldBag.listen(select, 'change', () => {
                        const state = select.value;
                        if (closed || ed.readonly || ed.runView || !inner.flush()) { syncPresence(); return; }
                        changed(prefix + field.name, state === 'absent' ? undefined : state === 'null' ? null : empty(field.type));
                        buildInner();
                        const next = Array.from(inner.el.querySelectorAll('.ed-field')).find(node => node.dataset.param === prefix + field.name);
                        if (next) next.querySelector('[data-ed-block-presence]').focus();
                    });
                }
                select.value = !own(data, field.name) ? 'absent' : data[field.name] === null ? 'null' : 'value';
                select.disabled = !!(ed.readonly || ed.runView);
                host.querySelector('.ed-field-control').hidden = select.value !== 'value';
                const mode = host.querySelector('[data-ed-mode]'); if (mode) mode.hidden = select.value !== 'value';
                const wrong = own(data, field.name) && !valid(field);
                select.setAttribute('aria-invalid', String(wrong));
                let error = host.querySelector('[data-ed-block-type-error]');
                if (!error) { error = core.el('<p class="ed-error" data-ed-block-type-error hidden></p>'); host.appendChild(error); }
                error.hidden = !wrong; error.textContent = wrong ? ed.t('easydrag.ui.error_flow_value_type') + ' ' + JSON.stringify(data[field.name]).slice(0, 512) : '';
            }
        }
        buildInner();
        return {
            el, values: () => { if (options.requireComplete) ED.transform.boundedJSON(data); return clone(data); }, value: name => data[name.slice(prefix.length)],
            mappingTargets: () => fields.map(field => ({ name: prefix + field.name, label: field.name, templatable: !!options.templatable })),
            insert(name, ref) {
                const field = fields.find(field => prefix + field.name === name);
                if (!field || closed || ed.readonly || ed.runView || !options.templatable) return;
                if (!own(data, field.name) || data[field.name] === null) { data[field.name] = empty(field.type); inner.refresh(synthetic(), options.roots || {}); syncPresence(); }
                inner.insert(name, ref);
            },
            focus: name => inner.focus(name),
            flush() {
                if (!inner.flush()) return false;
                if (rejected.size) { const error = Array.from(el.children).find(node => node.classList.contains('ed-error')); error.hidden = false; error.textContent = rejectionText(Array.from(rejected.values()).includes('incomplete') ? 'incomplete' : 'budget'); return false; }
                const missing = fields.some(field => field.required && !own(data, field.name)), wrong = fields.some(field => !valid(field)) || unknown().length > 0;
                const error = Array.from(el.children).find(node => node.classList.contains('ed-error')); error.hidden = !wrong; error.textContent = missing ? ed.t('easydrag.ui.block_required_values') : wrong ? ed.t('easydrag.ui.error_flow_value_type') : '';
                return !wrong || !options.requireComplete;
            },
            refresh(value, roots) { data = clone(value || {}); options.roots = roots || options.roots; inner.refresh(synthetic(), roots || {}); syncPresence(); showUnknown(); },
            dispose() { closed = true; fieldBag.dispose(); inner.dispose(); el.remove(); }
        };
    }
    function repairBoundary(env) {
        const { ed } = env, core = ED.core, node = env.node, observed = ed.model.version;
        let next, closed = false;
        const el = core.el('<div class="ed-block-boundary-repair"><p class="ed-error">' + ed.esc(ed.t('easydrag.ui.block_fields_invalid')) + '</p><p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_repair_hint')) + '</p><div data-ed-block-repair-editor></div><button type="button" class="ed-btn" data-ed-block-repair-apply disabled>' + ed.esc(ed.t('easydrag.ui.apply')) + '</button></div>');
        const apply = el.querySelector('[data-ed-block-repair-apply]');
        let initial;
        try { ED.transform.boundedJSON(node.params.fields); initial = node.params.fields; }
        catch (_) { el.insertBefore(core.el('<pre class="ed-code">' + ed.esc(ED.template.describe(node.params.fields, ed.t, core.fmt.number)) + ' …</pre>'), el.querySelector('[data-ed-block-repair-editor]')); }
        const editor = ED.fields.jsonEditor({ t: ed.t, esc: ed.esc, readonly: !!(ed.readonly || ed.runView), parseJSON: raw => { const value = ED.transform.readSample(raw); if (!validFields(value)) throw new Error('Invalid block fields'); return value; } }, initial, value => { next = value; apply.disabled = !!(ed.readonly || ed.runView || !validFields(next)); });
        const area = editor.querySelector('textarea'); area.setAttribute('aria-label', ed.t(node.type === 'block.input' ? 'easydrag.ui.block_inputs' : 'easydrag.ui.block_outputs'));
        el.querySelector('[data-ed-block-repair-editor]').appendChild(editor);
        apply.addEventListener('click', () => {
            if (closed || ed.readonly || ed.runView || ed.model.version !== observed || ed.model.node(node.id) !== node || !area.edCommit() || !validFields(next)) return;
            ed.model.endCoalescing();
            try { env.onChange('fields', clone(next), node.id); } finally { ed.model.endCoalescing(); }
            ed.bus.emit('schema');
        });
        return { el, flush: () => ed.readonly || ed.runView || !area.value || (next === undefined && area.getAttribute('aria-invalid') !== 'true'), mappingTargets: () => [], refresh() {}, dispose() { closed = true; el.remove(); } };
    }
    function boundary(env) {
        const el = ED.core.el('<div class="ed-block-boundary-host"></div>');
        let active, repair;
        function refresh(node, roots) {
            env.node = node; env.roots = roots;
            const invalid = !validFields(node.params.fields);
            if (!active || repair !== invalid) { if (active) active.dispose(); repair = invalid; active = invalid ? repairBoundary(env) : boundaryForm(env); el.appendChild(active.el); }
            else active.refresh(node, roots);
        }
        refresh(env.node, env.roots);
        return { el, flush: () => active.flush(), mappingTargets: () => active.mappingTargets(), insert: (name, ref) => active.insert && active.insert(name, ref), focus: name => active.focus && active.focus(name), value: name => active.value && active.value(name), refresh, dispose() { active.dispose(); el.remove(); } };
    }
    function boundaryForm(env) {
        const { ed } = env, core = ED.core, bag = core.bag(), id = env.node.id;
        let node = env.node, fields = clone(Array.isArray(node.params.fields) ? node.params.fields : []), origins = fields.map(field => field.name), form, closed = false, composing = new Set(), signature = '';
        const output = node.type === 'block.output';
        const el = core.el('<div class="ed-block-boundary"><p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_fields_hint')) + '</p><div data-ed-block-fields></div><button type="button" class="ed-btn" data-ed-block-field-add>' + core.icon('plus') + ed.esc(ed.t('easydrag.ui.fields_add')) + '</button><p class="ed-error" role="alert" hidden></p><div data-ed-block-bindings></div></div>');
        function current() { return !closed && !ed.readonly && !ed.runView && ed.model.node(id) === node; }
        function error(valid) { const box = el.querySelector('.ed-error'); box.hidden = valid; box.textContent = valid ? '' : ed.t('easydrag.ui.block_fields_invalid'); return valid; }
        function commit() {
            if (ed.readonly || ed.runView) return true;
            if (!current() || composing.size || !error(validFields(fields))) return false;
            if (form && !form.flush()) return false;
            const next = clone(fields);
            const bindings = Object.assign(Object.create(null), node.params.values || {});
            if (output) {
                next.forEach((field, index) => { const old = origins[index]; if (old && old !== field.name && own(bindings, old) && !own(bindings, field.name)) bindings[field.name] = bindings[old]; });
                (Array.isArray(node.params.fields) ? node.params.fields : []).forEach(field => { if (!next.some(nextField => nextField.name === field.name)) delete bindings[field.name]; });
            }
            signature = JSON.stringify(next);
            ed.model.change('block-contract', rec => { rec.node(id).params.fields = next; if (output) rec.node(id).params.values = bindings; });
            origins = next.map(field => field.name);
            node = ed.model.node(id); return true;
        }
        function buildBindings() {
            if (form) form.dispose(); form = null;
            if (!output || !validFields(fields)) return;
            form = values({ ed, nodeId: id, fields, value: node.params.values || {}, prefix: 'values.', templatable: true, roots: env.roots, upstream: env.upstream, onChange: next => { if (current()) env.onChange('values', next, id); } });
            el.querySelector('[data-ed-block-bindings]').appendChild(form.el);
        }
        function build() {
            signature = JSON.stringify(fields);
            el.querySelector('[data-ed-block-fields]').innerHTML = fields.map((field, index) => '<div class="ed-block-field" data-ed-block-field="' + index + '"><label class="ed-label">' + ed.esc(ed.t('easydrag.param.field_name')) + '<input class="ed-input" data-ed-block-key="name" maxlength="40" value="' + ed.esc(field.name) + '"' + (ed.readonly ? ' disabled' : '') + '></label><label class="ed-label">' + ed.esc(ed.t('easydrag.param.field_type')) + '<select class="ed-input" data-ed-block-key="type"' + (ed.readonly ? ' disabled' : '') + '>' + Object.keys(ED.blocks.types).map(type => '<option value="' + type + '"' + (field.type === type ? ' selected' : '') + '>' + ed.esc(ed.t('easydrag.ui.type_' + ED.blocks.types[type])) + '</option>').join('') + '</select></label>' + ['required', 'nullable'].map(key => '<label class="ed-check"><input type="checkbox" data-ed-block-key="' + key + '"' + (field[key] ? ' checked' : '') + (ed.readonly ? ' disabled' : '') + '> ' + ed.esc(ed.t('easydrag.ui.block_' + key)) + '</label>').join('') + '<button type="button" class="ed-icon-btn" data-ed-block-field-remove aria-label="' + ed.esc(ed.t('easydrag.ui.delete') + ' · ' + (index + 1)) + '"' + (ed.readonly ? ' disabled' : '') + '>' + core.icon('trash') + '</button></div>').join('');
            el.querySelectorAll('[data-ed-block-key]').forEach(input => { input.edCommit = commit; });
            el.querySelector('[data-ed-block-field-add]').disabled = !!ed.readonly || fields.length >= 64;
            buildBindings();
        }
        bag.listen(el, 'compositionstart', event => composing.add(event.target));
        bag.listen(el, 'compositionend', event => { composing.delete(event.target); read(event); });
        function read(event) {
            const row = event.target.closest('[data-ed-block-field]'), key = event.target.dataset.edBlockKey;
            if (!row || !key || !current()) return;
            fields[Number(row.dataset.edBlockField)][key] = event.target.getAttribute('type') === 'checkbox' ? event.target.checked : event.target.value;
        }
        bag.listen(el, 'input', read);
        bag.listen(el, 'change', event => { read(event); if (!event.target.closest('[data-ed-block-key]') || composing.size) return; if (commit()) { buildBindings(); ed.bus.emit('schema'); } });
        bag.listen(el, 'click', event => {
            if (!current() || composing.size) return;
            if (event.target.closest('[data-ed-block-field-add]')) {
                if (!commit() || fields.length >= 64) return;
                let n = fields.length + 1; while (fields.some(field => field.name === 'field_' + n)) n++;
                fields.push({ name: 'field_' + n, type: 'string', required: true, nullable: false });
                origins.push('');
                if (commit()) { build(); ed.bus.emit('schema'); }
            }
            const remove = event.target.closest('[data-ed-block-field-remove]');
            if (remove && commit()) { const at = Number(remove.closest('[data-ed-block-field]').dataset.edBlockField); fields.splice(at, 1); origins.splice(at, 1); if (commit()) { build(); ed.bus.emit('schema'); } }
        });
        build();
        return { el, flush: commit, insert: (name, ref) => form && form.insert(name, ref), focus: name => form && form.focus(name), value: name => node.params.values && node.params.values[name.slice('values.'.length)], mappingTargets: () => form ? form.mappingTargets() : [], refresh(next, roots) {
            node = next; env.roots = roots;
            if (JSON.stringify(next.params.fields || []) !== signature && !el.contains(document.activeElement) && !composing.size) { fields = clone(Array.isArray(next.params.fields) ? next.params.fields : []); origins = fields.map(field => field.name); build(); }
            else if (form) form.refresh(next.params.values || {}, roots);
        }, dispose() { closed = true; bag.dispose(); composing.clear(); if (form) form.dispose(); el.remove(); } };
    }
    function call(env) {
        const { ed } = env, core = ED.core, id = env.node.id, controller = new AbortController(), bag = core.bag();
        let node = env.node, form = null, batch = null, bindingShape = null, boundItem, bindingBag = core.bag(), closed = false, versionSeq = 0, bindingSeq = 0, reviewSeq = 0, listSeq = 0, listRows = [], listCursor = '', versionRows = [], versionCursor = 0, versionBusy = false, stagedId = node.params.flow_id || '', stagedRevision = Number(node.params.revision) || 0;
        const each = node.type === 'component.each';
        const el = core.el('<div class="ed-block-call"><p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_pins_hint')) + '</p><label class="ed-label">' + ed.esc(ed.t('easydrag.ui.blocks_title')) + '<select class="ed-input" data-ed-call-block></select></label><label class="ed-label">' + ed.esc(ed.t('easydrag.ui.block_revision')) + '<select class="ed-input" data-ed-call-version></select></label><button type="button" class="ed-btn" data-ed-call-apply>' + ed.esc(ed.t('easydrag.ui.block_use_version')) + '</button><p class="ed-error" role="alert" hidden></p><div data-ed-batch-options></div><div data-ed-call-inputs></div></div>');
        const blockSelect = el.querySelector('[data-ed-call-block]'), versionSelect = el.querySelector('[data-ed-call-version]'), apply = el.querySelector('[data-ed-call-apply]'), errorBox = el.querySelector('.ed-error');
        const selfWarning = core.el('<p class="ed-error" role="alert" data-ed-call-self hidden>' + ed.esc(ed.t('easydrag.ui.block_self_reference')) + '</p>');
        selfWarning.id = core.randomID('ed-call-self-', 8);
        el.insertBefore(selfWarning, blockSelect.closest('label'));
        if (ed.isBlock) el.insertBefore(core.el('<details class="ed-details" data-ed-nested-call-help><summary>' + ed.esc(ed.t('easydrag.ui.block_nested_title')) + '</summary><p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_nested_hint')) + '</p></details>'), selfWarning);
        const more = core.el('<button type="button" class="ed-btn" data-ed-call-more hidden>' + ed.esc(ed.t('easydrag.ui.block_load_more')) + '</button>'); el.insertBefore(more, versionSelect.closest('label'));
        const versionMore = core.el('<button type="button" class="ed-btn" data-ed-call-version-more hidden>' + ed.esc(ed.t('easydrag.ui.block_load_more')) + '</button>'); el.insertBefore(versionMore, apply);
        function current() { return !closed && ed.model.node(id) === node; }
        const selfChoice = () => !!ed.isBlock && stagedId === ed.flow.id;
        function applyState() { const self = selfChoice(); selfWarning.hidden = !self; blockSelect.setAttribute('aria-invalid', String(self)); if (self) blockSelect.setAttribute('aria-describedby', selfWarning.id); else blockSelect.removeAttribute('aria-describedby'); apply.disabled = !current() || !!(ed.readonly || ed.runView || !stagedRevision || self); }
        function blockChoices() {
            const rows = listRows.filter(row => !ed.isBlock || row.id !== ed.flow.id).map(row => ({ value: row.id, label: row.name }));
            if (stagedId && !rows.some(row => row.value === stagedId)) rows.push({ value: stagedId, label: stagedId, disabled: selfChoice() });
            options(blockSelect, rows, stagedId); applyState();
        }
        function fail(err) { errorBox.hidden = false; errorBox.textContent = err ? core.errorText(ed.t, err) : ed.t('easydrag.ui.block_contract_unavailable'); }
        function options(select, rows, value) { select.innerHTML = '<option value="">' + ed.esc(ed.t('easydrag.ui.option_none')) + '</option>' + rows.map(row => '<option value="' + ed.esc(row.value) + '"' + (row.disabled ? ' disabled' : '') + '>' + ed.esc(row.label) + '</option>').join(''); select.value = String(value || ''); select.disabled = !!(ed.readonly || ed.runView); }
        if (each) { batch = ED.batches.options(env); el.querySelector('[data-ed-batch-options]').appendChild(batch.el); }
        function renderBindings(shape) {
            bindingBag.dispose(); bindingBag = core.bag();
            if (form) form.dispose(); form = null;
            const host = el.querySelector('[data-ed-call-inputs]'); host.innerHTML = '';
            if (each) {
                const raw = node.params.item_field, selected = typeof raw === 'string' ? raw : ':invalid';
                const missing = own(node.params, 'item_field') && raw !== '' && !shape.inputs.some(field => field.name === raw);
                const item = core.el('<label class="ed-label">' + ed.esc(ed.t('easydrag.ui.batch_item_field')) + '<select class="ed-input" data-ed-batch-item-field></select></label>');
                host.appendChild(item); const select = item.querySelector('select');
                options(select, shape.inputs.map(field => ({ value: field.name, label: field.name + ' · ' + ed.t('easydrag.ui.type_' + ED.blocks.types[field.type]) })), typeof raw === 'string' ? raw : '');
                if (missing) { const label = typeof raw === 'string' ? raw.slice(0, 80) : ED.template.describe(raw, ed.t, core.fmt.number); select.appendChild(core.el('<option value="' + ed.esc(selected) + '">' + ed.esc(ed.t('easydrag.ui.batch_item_missing', { field: label })) + '</option>')); select.value = selected; select.setAttribute('aria-invalid', 'true'); }
                bindingBag.listen(select, 'change', () => { if ((select.value === '' || shape.inputs.some(field => field.name === select.value)) && current() && !ed.readonly && !ed.runView && (!form || form.flush()) && batch.flush()) env.onChange('item_field', select.value, id); });
                host.appendChild(core.el('<h4>' + ed.esc(ed.t('easydrag.ui.batch_shared_inputs')) + '</h4><p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.batch_shared_hint')) + '</p>'));
            }
            boundItem = node.params.item_field;
            form = values({ ed, nodeId: id, fields: shape.inputs.filter(field => !each || field.name !== node.params.item_field), value: node.params.inputs || {}, prefix: 'inputs.', templatable: true, roots: env.roots, upstream: env.upstream, onChange: next => { if (current() && !ed.readonly && !ed.runView) env.onChange('inputs', next, id); } });
            host.appendChild(form.el); ed.bus.emit('schema');
        }
        async function versions(append) {
            if (append && versionBusy) return;
            const mine = ++versionSeq, selectedId = stagedId; apply.disabled = true;
            versionBusy = true; versionMore.disabled = true;
            try {
                const res = await ed.blockApi.versions(selectedId, append ? versionCursor : 0, controller.signal);
                if (!current() || mine !== versionSeq || selectedId !== stagedId) return;
                versionRows = Array.from(new Map((append ? versionRows.concat(res.versions || []) : res.versions || []).map(row => [row.revision, row])).values()); versionCursor = res.next_cursor || 0;
                const rows = versionRows.map(row => ({ value: row.revision, label: 'v' + row.revision }));
                if (stagedRevision && !rows.some(row => Number(row.value) === stagedRevision)) rows.push({ value: stagedRevision, label: 'v' + stagedRevision });
                options(versionSelect, rows, stagedRevision); versionMore.hidden = !versionCursor; applyState();
            }
            catch (err) { if (current() && mine === versionSeq) fail(err); }
            finally { if (mine === versionSeq) { versionBusy = false; versionMore.disabled = false; } }
        }
        async function bindings() {
            const mine = ++bindingSeq;
            bindingBag.dispose(); bindingBag = core.bag();
            if (form) form.dispose(); form = null; bindingShape = null; el.querySelector('[data-ed-call-inputs]').innerHTML = '';
            if (!node.params.flow_id || !node.params.revision) { ed.bus.emit('schema'); return; }
            const refId = node.params.flow_id, refRevision = node.params.revision;
            try {
                const res = await ed.blockApi.version(refId, refRevision, controller.signal), shape = ED.blocks.contract(res.contract);
                if (!current() || mine !== bindingSeq || node.params.flow_id !== refId || node.params.revision !== refRevision) return;
                ed.catalog.blockContracts.set(refId + ':' + refRevision, shape);
                bindingShape = shape; renderBindings(shape);
            } catch (err) { if (current() && mine === bindingSeq) { fail(err); ed.bus.emit('schema'); } }
        }
        bag.listen(blockSelect, 'change', () => { if (!current() || ed.readonly || ed.runView) return; stagedId = blockSelect.value; stagedRevision = 0; options(versionSelect, [], ''); applyState(); if (stagedId) versions(); });
        bag.listen(versionSelect, 'change', () => { stagedRevision = Number(versionSelect.value); applyState(); });
        bag.listen(versionMore, 'click', () => { if (versionCursor) versions(true); });
        bag.listen(apply, 'click', async () => {
            if (!current() || ed.readonly || ed.runView || !stagedId || !stagedRevision || selfChoice() || ed.flushPendingEdits() === false) return;
            const observed = ed.model.version, refId = stagedId, revision = stagedRevision, review = ++reviewSeq;
            apply.disabled = true;
            try {
                const res = await ed.blockApi.version(refId, revision, controller.signal), shape = ED.blocks.contract(res.contract);
                const deps = node.params.flow_id ? await ed.blockApi.dependents(refId, '', controller.signal) : { items: [] };
                if (!current() || review !== reviewSeq || refId !== stagedId || revision !== stagedRevision || ed.model.version !== observed || ed.readonly || ed.runView) return;
                const before = ed.catalog.blockContracts.get(node.params.flow_id + ':' + node.params.revision);
                const retained = Object.fromEntries(shape.inputs.filter(field => own(node.params.inputs || {}, field.name)).map(field => [field.name, node.params.inputs[field.name]]));
                const removed = Object.keys(node.params.inputs || {}).filter(name => !shape.inputs.some(field => field.name === name));
                const dialog = core.modal(ed.root, { title: ed.t('easydrag.ui.block_use_version'), closeLabel: ed.t('easydrag.ui.close'), className: 'ed-modal--block', body: '<p>' + ed.esc(ed.t('easydrag.ui.block_upgrade_hint')) + '</p><p><code>' + ed.esc(refId) + ' · v' + revision + '</code></p>' + (before ? '<h3>' + ed.esc(ed.t('easydrag.ui.publish_change_before')) + '</h3>' + ED.blocks.contractMarkup(ed, before) : '') + '<h3>' + ed.esc(ed.t('easydrag.ui.publish_change_after')) + '</h3>' + ED.blocks.contractMarkup(ed, shape) + (each && node.params.item_field && !shape.inputs.some(field => field.name === node.params.item_field) ? '<p class="ed-callout ed-callout--warn">' + ed.esc(ed.t('easydrag.ui.batch_item_missing', { field: typeof node.params.item_field === 'string' ? node.params.item_field.slice(0, 80) : ED.template.describe(node.params.item_field, ed.t, core.fmt.number) })) + '</p>' : '') + (removed.length ? '<p class="ed-callout">' + ed.esc(ed.t('easydrag.ui.block_removed_bindings', { fields: removed.join(', ') })) + '</p>' : '') + '<p class="ed-hint">' + ed.esc(ed.t('easydrag.ui.block_dependencies')) + ': ' + ed.esc((deps.items || []).length) + (deps.next_cursor ? '+' : '') + '</p>', actions: [{ id: 'cancel', label: ed.t('easydrag.ui.cancel') }, { id: 'apply', label: ed.t('easydrag.ui.block_use_version'), primary: true }], onAction: action => {
                    if (action !== 'apply') return true;
                    if (!current() || review !== reviewSeq || refId !== stagedId || revision !== stagedRevision || ed.model.version !== observed || ed.readonly || ed.runView || selfChoice() || ed.flushPendingEdits() === false || ed.model.version !== observed) return false;
                    ed.catalog.blockContracts.set(refId + ':' + revision, shape);
                    ed.model.change('block-reference', rec => { const next = rec.node(id); next.params = Object.assign({}, each ? next.params : {}, { flow_id: refId, revision, inputs: retained }); next.settings.retry = { count: 0 }; });
                    const latest = ed.model.node(id); if (node !== latest) { node = latest; bindings(); } return true;
                } });
                dialog.body.appendChild(core.el('<p class="ed-hint" data-ed-block-dependencies-hint>' + ed.esc(ed.t('easydrag.ui.block_dependencies_hint')) + '</p>'));
                bag.add(() => dialog.close(null));
            } catch (err) { if (current() && err.name !== 'AbortError') fail(err); }
            finally { if (current()) applyState(); }
        });
        async function loadList(append) {
            const mine = ++listSeq; more.disabled = true;
            try { const res = await ed.blockApi.list(append ? listCursor : '', controller.signal); if (!current() || mine !== listSeq) return; listRows = Array.from(new Map((append ? listRows.concat(res.blocks || []) : res.blocks || []).map(row => [row.id, row])).values()); listCursor = res.next_cursor || ''; blockChoices(); more.hidden = !listCursor; }
            catch (err) { if (current() && mine === listSeq && err.name !== 'AbortError') fail(err); }
            finally { if (current() && mine === listSeq) more.disabled = false; }
        }
        bag.listen(more, 'click', () => { if (listCursor) loadList(true); });
        loadList(false); if (stagedId) versions(); bindings();
        applyState(); apply.disabled = true;
        const owner = name => batch && !name.startsWith('inputs.') ? batch : form;
        return {
            el, insert: (name, ref) => owner(name) && owner(name).insert(name, ref),
            focus(name) { const target = name === 'flow_id' ? blockSelect : name === 'revision' ? versionSelect : name === 'item_field' ? el.querySelector('[data-ed-batch-item-field]') : null; if (target) target.focus(); else if (owner(name)) owner(name).focus(name); },
            flush: () => (!batch || batch.flush()) && (!form || form.flush()),
            value: name => name === 'items' ? node.params.items : node.params.inputs && node.params.inputs[name.slice('inputs.'.length)],
            mappingTargets: () => (batch ? batch.mappingTargets() : []).concat(form ? form.mappingTargets() : []),
            refresh(next, roots) {
                const refChanged = next.params.flow_id !== node.params.flow_id || next.params.revision !== node.params.revision;
                node = next; env.roots = roots; if (batch) batch.refresh(next, roots);
                if (refChanged) { reviewSeq++; stagedId = node.params.flow_id || ''; stagedRevision = Number(node.params.revision) || 0; blockChoices(); versions(); bindings(); }
                else if (each && bindingShape && boundItem !== node.params.item_field) renderBindings(bindingShape);
                else if (form) form.refresh(node.params.inputs || {}, roots);
            },
            dispose() { closed = true; versionSeq++; bindingSeq++; reviewSeq++; controller.abort(); if (form) form.dispose(); if (batch) batch.dispose(); bindingBag.dispose(); bag.dispose(); el.remove(); }
        };
    }
    function handles(type) { return ['block.input', 'block.output', 'component.call', 'component.each'].includes(type); }
    // Metadata reads touch only changed call IDs after the initial document scan, never run ticks.
    function track(ed) {
        const controller = new AbortController(), queue = [], seen = new Set(), refs = new Map(), nodes = new Map(), requests = new Map();
        let active = 0, closed = false;
        ed.catalog.blockContracts.clear();
        function update(id) {
            const node = ed.model.node(id), old = nodes.get(id);
            const key = ed.blockApi && node && ['component.call', 'component.each'].includes(node.type) && typeof node.params.flow_id === 'string' && Number.isInteger(node.params.revision) && node.params.revision > 0 ? node.params.flow_id + ':' + node.params.revision : '';
            if (old === key) return;
            if (old) { const count = refs.get(old) - 1; if (count) refs.set(old, count); else { refs.delete(old); seen.delete(old); requests.delete(old); ed.catalog.blockContracts.delete(old); } }
            nodes.delete(id);
            if (!key) return;
            nodes.set(id, key); refs.set(key, (refs.get(key) || 0) + 1);
            if (seen.has(key)) return;
            seen.add(key);
            if (!ed.catalog.blockContracts.has(key)) { const ticket = {}; requests.set(key, ticket); queue.push({ id: node.params.flow_id, revision: node.params.revision, key, ticket, cached: ed.catalog.blockContracts.get(key) }); }
            drain();
        }
        function drain() {
            while (!closed && active < 4 && queue.length) {
                const ref = queue.shift(); active++;
                ed.blockApi.version(ref.id, ref.revision, controller.signal).then(res => {
                    if (!closed && requests.get(ref.key) === ref.ticket && refs.has(ref.key)) ed.catalog.blockContracts.set(ref.key, ED.blocks.contract(res.contract));
                }).catch(() => { if (!closed && requests.get(ref.key) === ref.ticket && refs.has(ref.key) && ed.catalog.blockContracts.get(ref.key) === ref.cached) ed.catalog.blockContracts.delete(ref.key); }).finally(() => { active--; if (!closed) { if (requests.get(ref.key) === ref.ticket) ed.bus.emit('schema'); drain(); } });
            }
        }
        ed.model.doc.nodes.forEach(node => update(node.id));
        const off = ed.bus.on('model', change => {
            if (change.kind === 'reset') { refs.clear(); nodes.clear(); seen.clear(); requests.clear(); queue.length = 0; ed.catalog.blockContracts.clear(); ed.model.doc.nodes.forEach(node => update(node.id)); }
            else (change.nodes || []).forEach(update);
        });
        return { dispose() { closed = true; controller.abort(); queue.length = 0; seen.clear(); refs.clear(); nodes.clear(); requests.clear(); ed.catalog.blockContracts.clear(); off(); } };
    }
    ED.blockFields = { values, handles, track, render: env => ['component.call', 'component.each'].includes(env.node.type) ? call(env) : boundary(env) };
})();
