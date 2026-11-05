// Guided template filters: local bounded previews and one existing caret-aware insertion.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const MAX_CHAIN = 8, MAX_BYTES = 64 * 1024, MAX_DISPLAY = 8 * 1024;
    const arg = (kind, key, value, extra) => Object.assign({ kind, key, value }, extra);
    const FIELD = 'easydrag.param.field_name', VALUE = 'easydrag.param.field_value';
    const SPECS = {
        default: [arg('literal', VALUE, '')],
        truncate: [arg('number', 'easydrag.ui.transform_characters', 100, { min: 1, max: 2147483647 })],
        upper: [], lower: [], trim: [],
        join: [arg('text', 'easydrag.ui.transform_separator', ', ')],
        split: [arg('text', 'easydrag.ui.transform_separator', ',')],
        first: [], last: [], count: [],
        pluck: [arg('text', FIELD, 'title')], json: [],
        date: [arg('text', 'easydrag.ui.transform_date_format', 'YYYY-MM-DD'), arg('zone', 'easydrag.ui.transform_timezone', 'UTC')],
        round: [arg('number', 'easydrag.ui.transform_decimals', 0, { min: 0, max: 10 })],
        replace: [arg('text', 'easydrag.ui.transform_find', ''), arg('text', 'easydrag.ui.transform_replacement', '')],
        strip_html: [],
        pick: [arg('text', FIELD, 'name')],
        rename: [arg('text', 'easydrag.ui.transform_from_field', 'old'), arg('text', 'easydrag.ui.transform_to_field', 'new')],
        where: [arg('text', FIELD, 'value'), arg('choice', 'easydrag.ui.transform_comparison', 'eq', { options: [
            ['eq', 'easydrag.ui.op_eq'], ['ne', 'easydrag.ui.op_ne'], ['lt', 'easydrag.ui.op_lt'], ['le', 'easydrag.ui.op_lte'], ['gt', 'easydrag.ui.op_gt'], ['ge', 'easydrag.ui.op_gte']
        ] }), arg('literal', VALUE, '')],
        sort: [arg('text', 'easydrag.ui.transform_field_optional', ''), arg('choice', 'easydrag.ui.transform_order', 'asc', { options: [['asc', 'easydrag.ui.transform_ascending'], ['desc', 'easydrag.ui.transform_descending']] })],
        unique: [arg('text', 'easydrag.ui.transform_field_optional', '')],
        sum: [arg('text', 'easydrag.ui.transform_field_optional', '')]
    };
    function fieldsFor(call) { return call.name === 'pick' ? call.args.map(() => SPECS.pick[0]) : SPECS[call.name]; }
    function fail(kind) { throw Object.assign(new Error(kind), { transformKind: kind }); }
    function size(text, limit = Infinity) {
        if (text.length > limit) return limit + 1;
        let bytes = 0;
        for (const point of text) { const n = point.codePointAt(0); bytes += n < 128 ? 1 : n < 2048 ? 2 : n < 65536 ? 3 : 4; if (bytes > limit) break; }
        return bytes;
    }
    function jsonStringSize(text, limit) {
        if (text.length + 2 > limit) return limit + 1;
        let bytes = 2;
        for (const point of text) {
            const n = point.codePointAt(0);
            bytes += n === 34 || n === 92 || [8, 9, 10, 12, 13].includes(n) ? 2 : n < 32 || n >= 0xD800 && n <= 0xDFFF ? 6 : n < 128 ? 1 : n < 2048 ? 2 : n < 65536 ? 3 : 4;
            if (bytes > limit) break;
        }
        return bytes;
    }
    function readSample(text, emptyKind = 'no_sample') {
        if (typeof text !== 'string' || size(text, MAX_BYTES) > MAX_BYTES) fail('budget');
        if (!text.trim()) fail(emptyKind);
        return JSON.parse(text);
    }
    function bounded(value) {
        const stack = [{ value, depth: 0 }], seen = new Set();
        let count = 0, bytes = 0;
        const add = amount => { bytes += amount; if (bytes > MAX_BYTES) fail('budget'); };
        while (stack.length) {
            const item = stack.pop();
            if (++count > 10000 || item.depth > 32) fail('budget');
            const current = item.value;
            if (current === null) { add(4); continue; }
            if (typeof current === 'string') { add(jsonStringSize(current, MAX_BYTES - bytes)); continue; }
            if (typeof current === 'boolean') { add(current ? 4 : 5); continue; }
            if (typeof current === 'number' && Number.isFinite(current)) { add(String(current).length); continue; }
            if (current === undefined) fail('no_sample');
            if (typeof current !== 'object' || typeof current.toJSON === 'function') fail('invalid');
            if (seen.has(item.value)) fail('invalid');
            seen.add(item.value);
            add(2);
            if (Array.isArray(current)) {
                if (current.length + count + stack.length > 10000) fail('budget');
                add(Math.max(0, current.length - 1));
                for (const child of current) stack.push({ value: child, depth: item.depth + 1 });
            } else {
                let keys = 0;
                for (const key in current) {
                    if (!Object.prototype.hasOwnProperty.call(current, key)) continue;
                    if (++keys + count + stack.length > 10000) fail('budget');
                    add((keys > 1 ? 1 : 0) + jsonStringSize(key, MAX_BYTES - bytes) + 1);
                    stack.push({ value: current[key], depth: item.depth + 1 });
                }
            }
        }
        const json = JSON.stringify(value);
        if (json === undefined) fail('no_sample');
        if (size(json) > MAX_BYTES) fail('budget');
        if (ED.testing) { try { ED.testing.parseSample('{"value":' + json + '}'); } catch (_) { fail('incomplete'); } }
        return json;
    }
    function numberLiteral(value) {
        if (Object.is(value, -0)) return '-0';
        const text = String(value);
        if (!text.includes('e')) return text;
        const [coefficient, exponent] = text.split('e'), negative = coefficient[0] === '-';
        const unsigned = negative ? coefficient.slice(1) : coefficient, digits = unsigned.replace('.', '');
        const dot = (unsigned.includes('.') ? unsigned.indexOf('.') : unsigned.length) + Number(exponent);
        const body = dot <= 0 ? '0.' + '0'.repeat(-dot) + digits : dot >= digits.length ? digits + '0'.repeat(dot - digits.length) : digits.slice(0, dot) + '.' + digits.slice(dot);
        return (negative ? '-' : '') + body;
    }
    function literal(value) {
        if (typeof value === 'string') {
            if (value.length > 4096 || /[\uD800-\uDFFF]/u.test(value)) fail('invalid');
            // Exactly pathJoin's lexer escapes: raw CR/Unicode stay raw; only LF/tab have escapes.
            return '"' + value.replace(/[\\"]/g, point => '\\' + point).replace(/\n/g, '\\n').replace(/\t/g, '\\t') + '"';
        }
        if (value === null || typeof value === 'boolean') return String(value);
        if (typeof value === 'number' && Number.isFinite(value)) return numberLiteral(value);
        fail('invalid');
    }
    function available() { return ED.template.FILTER_LIST.filter(filter => Object.prototype.hasOwnProperty.call(SPECS, filter.name)); }
    function initial(name) { return { name, args: SPECS[name].map(field => field.value) }; }
    function build(ref, chain) {
        if (typeof ref !== 'string' || ref.length > 4096 || !chain.length || chain.length > MAX_CHAIN) fail('invalid');
        const source = ED.template.parseExpr(ref);
        if (source.filters.length) fail('invalid');
        const names = new Set(available().map(filter => filter.name));
        const suffix = chain.map(call => {
            if (!names.has(call.name) || !Array.isArray(call.args) || (call.name === 'pick' ? call.args.length < 1 || call.args.length > 16 : call.args.length !== SPECS[call.name].length)) fail('invalid');
            fieldsFor(call).forEach((field, i) => {
                const value = call.args[i];
                if (field.kind === 'number' && (typeof value !== 'number' || !Number.isInteger(value) || value < field.min || value > field.max)) fail('invalid');
                if (['text', 'zone', 'choice'].includes(field.kind) && typeof value !== 'string') fail('invalid');
                if (field.options && !field.options.some(option => option[0] === value)) fail('invalid');
                if (field.kind === 'zone' && !value) fail('invalid');
            });
            return ' | ' + call.name + (call.args.length ? '(' + call.args.map(literal).join(', ') + ')' : '');
        }).join('');
        const expression = ref + suffix;
        if (expression.length > 8192) fail('invalid');
        const parsed = ED.template.parseExpr(expression);
        if (JSON.stringify(parsed.filters.map(call => call.args)) !== JSON.stringify(chain.map(call => call.args))) fail('invalid');
        return expression;
    }
    function occurrences(text, separator) {
        if (!separator) return 0;
        let count = 0, at = 0;
        while ((at = text.indexOf(separator, at)) >= 0) { count++; at += separator.length; }
        return count;
    }
    function estimate(call, value) {
        if (call.name === 'replace') {
            const text = ED.template.stringify(value), [from, to] = call.args;
            if (from && size(text) + occurrences(text, from) * (size(to) - size(from)) > MAX_BYTES) fail('budget');
        }
        if (call.name === 'join' && Array.isArray(value)) {
            let bytes = Math.max(0, value.length - 1) * size(call.args[0]);
            if (bytes > MAX_BYTES) fail('budget');
            for (const item of value) { bytes += size(ED.template.stringify(item)); if (bytes > MAX_BYTES) fail('budget'); }
        }
        if (call.name === 'split') {
            const text = ED.template.stringify(value), separator = call.args[0];
            let parts = separator ? occurrences(text, separator) + 1 : 0;
            if (!separator) for (const _ of text) parts++;
            if (size(JSON.stringify(text)) + parts * 3 > MAX_BYTES) fail('budget');
        }
        if (call.name === 'date' && typeof value === 'string' && !/(?:Z|[+-]\d\d:\d\d)$/i.test(value.trim())) fail('date');
    }
    function preview(ref, chain, value) {
        const expression = build(ref, chain), parsed = ED.template.parseExpr(expression);
        bounded(value);
        let output = value;
        for (const call of parsed.filters) {
            estimate(call, output);
            output = ED.template.evaluate('{{' + build('preview_value', [call]) + '}}', { preview_value: output });
            bounded(output);
        }
        const full = ED.template.stringify(output);
        let text = '', bytes = 0;
        for (const point of full) { const next = size(point); if (bytes + next > MAX_DISPLAY) break; text += point; bytes += next; }
        return { value: output, text, limited: text.length < full.length, expression };
    }

    function open(ed, env) {
        const core = ED.core, { t, esc } = ed;
        if (ed.readonly || ed.runView || !env.isCurrent()) return null;
        const chain = [initial('trim')], composing = new Set();
        let dialog, sample = '', sampleError = '', hasSample = env.hasValue, closed = false;
        try { if (hasSample) sample = bounded(env.value); } catch (err) { hasSample = false; sampleError = err.transformKind || 'invalid'; }
        const zones = ['UTC'];
        try { if (typeof Intl.supportedValuesOf === 'function') zones.push(...Intl.supportedValuesOf('timeZone').filter(zone => zone !== 'UTC')); } catch (_) { /* UTC remains an explicit supported choice. */ }
        dialog = core.modal(ed.root, {
            title: t('easydrag.ui.transform_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--transform',
            body: [
                '<p class="ed-hint">' + esc(t('easydrag.ui.transform_hint')) + '</p>',
                '<div class="ed-transform-context"><p><strong>' + esc(t('easydrag.ui.transform_source')) + '</strong> <code>' + esc(env.ref) + '</code></p><p class="ed-hint" data-ed-transform-provenance>' + esc(env.sourceLabel || t('easydrag.ui.input_source')) + '</p><p><strong>' + esc(t('easydrag.ui.mapping_target')) + '</strong> ' + esc(env.targetLabel) + '</p></div>',
                '<p class="ed-hint" data-ed-transform-literals hidden>' + esc(t('easydrag.ui.hint_plaintext_value')) + '</p><ol class="ed-transform-chain" data-ed-transform-chain></ol>',
                '<button type="button" class="ed-btn" data-ed-transform-action="add">' + core.icon('plus') + esc(t('easydrag.ui.transform_add')) + '</button>',
                '<details class="ed-details"' + (hasSample ? '' : ' open') + '><summary>' + esc(t('easydrag.ui.transform_sample')) + '</summary><p class="ed-hint">' + esc(t('easydrag.ui.transform_sample_hint')) + '</p><textarea class="ed-input ed-code" rows="4" aria-label="' + esc(t('easydrag.ui.transform_sample')) + '" data-ed-transform-sample></textarea></details>',
                '<h4>' + esc(t('easydrag.ui.transform_preview')) + '</h4><p class="ed-hint" data-ed-transform-parity hidden>' + esc(t('easydrag.ui.transform_preview_hint')) + '</p><p class="ed-callout" data-ed-transform-preview-error hidden></p><pre class="ed-code ed-output-json" data-ed-transform-preview></pre>',
                '<details class="ed-details"><summary>' + esc(t('easydrag.ui.transform_notes')) + '</summary><p class="ed-hint">' + esc(t('easydrag.ui.transform_limit')) + '</p></details>',
                '<details class="ed-details"><summary>' + esc(t('easydrag.ui.transform_expression')) + '</summary><pre class="ed-code ed-output-json" data-ed-transform-expression></pre></details><p class="ed-error" role="alert" hidden></p>'
            ].join(''),
            actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'apply', label: t('easydrag.ui.transform_apply'), primary: true }],
            onAction(action, view) {
                if (action !== 'apply') return true;
                if (composing.size || !env.isCurrent() || ed.readonly || ed.runView) { error('stale'); return false; }
                try {
                    const expression = build(env.ref, chain);
                    if (env.apply(expression) === false) { error('stale'); return false; }
                    return true;
                } catch (_) { error('invalid'); return false; }
            }
        });
        const body = dialog.body, area = body.querySelector('[data-ed-transform-sample]');
        area.value = sample;
        function error(kind) { const el = body.querySelector('.ed-error'); el.hidden = false; el.textContent = t(kind === 'stale' ? 'easydrag.ui.transform_stale' : 'easydrag.ui.transform_invalid'); }
        function updatePreview() {
            if (closed || composing.size) return;
            const failure = body.querySelector('[data-ed-transform-preview-error]'), result = body.querySelector('[data-ed-transform-preview]');
            failure.hidden = true; result.textContent = '';
            let expression;
            try { expression = build(env.ref, chain); }
            catch (_) { dialog.el.querySelector('[data-ed-action="apply"]').disabled = true; error('invalid'); return; }
            dialog.el.querySelector('[data-ed-action="apply"]').disabled = false;
            body.querySelector('[data-ed-transform-expression]').textContent = '{{' + expression + '}}';
            body.querySelector('.ed-error').hidden = true;
            try {
                const value = readSample(area.value, sampleError || 'no_sample');
                const shown = preview(env.ref, chain, value);
                result.textContent = shown.text + (shown.limited ? '…' : '');
                if (shown.limited) { failure.hidden = false; failure.textContent = t('easydrag.ui.transform_budget'); }
            } catch (err) {
                failure.hidden = false;
                const key = { budget: 'easydrag.ui.transform_budget', no_sample: 'easydrag.ui.transform_no_sample', incomplete: 'easydrag.ui.transform_incomplete', date: 'easydrag.ui.transform_date_qualified' }[err.transformKind] || 'easydrag.ui.transform_preview_unavailable';
                failure.textContent = t(key);
            }
        }
        const schedule = core.debounce(updatePreview, 180);
        function renderRows() {
            body.querySelector('[data-ed-transform-chain]').innerHTML = chain.map((call, index) => '<li class="ed-transform-row" data-ed-transform-index="' + index + '"><label class="ed-label">' + esc(t('easydrag.ui.transform_operation')) + ' ' + (index + 1) + '<select class="ed-input" data-ed-transform-filter>' + available().map(filter => '<option value="' + filter.name + '"' + (filter.name === call.name ? ' selected' : '') + '>' + esc(core.tr(t, 'easydrag.ui.transform_filter_' + filter.name, filter.name)) + '</option>').join('') + '</select></label>' + fieldsFor(call).map((field, argIndex) => {
                const value = call.args[argIndex], type = value === null ? 'null' : typeof value === 'boolean' ? 'bool' : typeof value === 'number' ? 'number' : 'text';
                const choice = field.kind === 'literal' ? '<label class="ed-label">' + esc(t('easydrag.param.field_type')) + '<select class="ed-input" data-ed-transform-type="' + argIndex + '">' + ['text', 'number', 'bool', 'null'].map(kind => '<option value="' + kind + '"' + (kind === type ? ' selected' : '') + '>' + esc(core.tr(t, 'easydrag.ui.type_' + kind, kind)) + '</option>').join('') + '</select></label>' : '';
                const number = field.kind === 'number' || field.kind === 'literal' && type === 'number';
                const control = field.options ? '<select class="ed-input" data-ed-transform-arg="' + argIndex + '">' + field.options.map(option => '<option value="' + option[0] + '"' + (option[0] === value ? ' selected' : '') + '>' + esc(t(option[1])) + '</option>').join('') + '</select>' : field.kind === 'zone' ? '<select class="ed-input" data-ed-transform-arg="' + argIndex + '">' + zones.map(zone => '<option value="' + esc(zone) + '"' + (zone === value ? ' selected' : '') + '>' + esc(zone) + '</option>').join('') + '</select>' : field.kind === 'literal' && type === 'null' ? '<code>null</code>' : field.kind === 'literal' && type === 'bool' ? '<select class="ed-input" data-ed-transform-arg="' + argIndex + '"><option value="true"' + (value ? ' selected' : '') + '>true</option><option value="false"' + (!value ? ' selected' : '') + '>false</option></select>' : number ? '<input class="ed-input" type="number" step="' + (field.kind === 'literal' ? 'any' : '1') + '" data-ed-transform-arg="' + argIndex + '" value="' + esc(value) + '"' + (field.min !== undefined ? ' min="' + field.min + '" max="' + field.max + '"' : '') + '>' : '<textarea class="ed-input" rows="2" data-ed-transform-arg="' + argIndex + '">' + esc(value) + '</textarea>';
                return choice + '<label class="ed-label">' + esc(t(field.key)) + (call.name === 'pick' ? ' ' + (argIndex + 1) : '') + control + '</label>' + (call.name === 'pick' ? '<button type="button" class="ed-icon-btn" data-ed-transform-action="arg-remove" data-ed-transform-arg-index="' + argIndex + '" aria-label="' + esc(t('easydrag.ui.delete') + ' · ' + (argIndex + 1)) + '"' + (call.args.length === 1 ? ' disabled' : '') + '>' + core.icon('trash') + '</button>' : '');
            }).join('') + (call.name === 'pick' ? '<button type="button" class="ed-btn" data-ed-transform-action="arg-add"' + (call.args.length >= 16 ? ' disabled' : '') + '>' + esc(t('easydrag.ui.fields_add')) + '</button>' : '') + '<div class="ed-transform-row-actions">' + ['up', 'down', 'remove'].map(action => '<button type="button" class="ed-icon-btn" data-ed-transform-action="' + action + '" aria-label="' + esc(t(action === 'up' ? 'easydrag.ui.move_up' : action === 'down' ? 'easydrag.ui.move_down' : 'easydrag.ui.delete') + ' · ' + (index + 1)) + '"' + (action === 'up' && index === 0 || action === 'down' && index === chain.length - 1 || action === 'remove' && chain.length === 1 ? ' disabled' : '') + '>' + core.icon(action === 'up' ? 'chevron-up' : action === 'down' ? 'chevron-down' : 'trash') + '</button>').join('') + '</div></li>').join('');
            body.querySelector('[data-ed-transform-action="add"]').disabled = chain.length >= MAX_CHAIN;
            body.querySelector('[data-ed-transform-parity]').hidden = !chain.some(call => ['date', 'strip_html', 'upper', 'lower', 'round'].includes(call.name));
            body.querySelector('[data-ed-transform-literals]').hidden = !chain.some(call => call.args.length);
            updatePreview();
        }
        function change(event) {
            if (closed) return;
            const row = event.target.closest('[data-ed-transform-index]');
            if (!row) {
                if (event.target === area) { sampleError = ''; body.querySelector('[data-ed-transform-provenance]').textContent = t('easydrag.ui.transform_sample'); }
                schedule(); return;
            }
            const index = Number(row.dataset.edTransformIndex), call = chain[index];
            if (event.target.matches('[data-ed-transform-filter]')) { chain[index] = initial(event.target.value); renderRows(); return; }
            if (event.target.matches('[data-ed-transform-type]')) { call.args[Number(event.target.dataset.edTransformType)] = { text: '', number: 0, bool: false, null: null }[event.target.value]; renderRows(); return; }
            if (event.target.matches('[data-ed-transform-arg]')) {
                const i = Number(event.target.dataset.edTransformArg), field = fieldsFor(call)[i];
                call.args[i] = field.kind === 'number' || field.kind === 'literal' && typeof call.args[i] === 'number' ? event.target.value.length <= 4096 && event.target.value.trim() ? Number(event.target.value) : NaN : field.kind === 'literal' && typeof call.args[i] === 'boolean' ? event.target.value === 'true' : event.target.value;
                schedule();
            }
        }
        body.addEventListener('input', change); body.addEventListener('change', change);
        body.addEventListener('compositionstart', event => composing.add(event.target));
        body.addEventListener('compositionend', event => { composing.delete(event.target); change(event); });
        body.addEventListener('click', event => {
            if (closed) return;
            const button = event.target.closest('[data-ed-transform-action]');
            if (!button || composing.size || button.disabled) return;
            const action = button.dataset.edTransformAction;
            if (action === 'add') { if (chain.length < MAX_CHAIN) chain.push(initial('trim')); renderRows(); return; }
            const row = button.closest('[data-ed-transform-index]'), at = Number(row.dataset.edTransformIndex);
            if (action === 'arg-add' && chain[at].name === 'pick' && chain[at].args.length < 16) chain[at].args.push('');
            if (action === 'arg-remove' && chain[at].name === 'pick' && chain[at].args.length > 1) chain[at].args.splice(Number(button.dataset.edTransformArgIndex), 1);
            if (action === 'remove' && chain.length > 1) chain.splice(at, 1);
            if (action === 'up' && at > 0) [chain[at - 1], chain[at]] = [chain[at], chain[at - 1]];
            if (action === 'down' && at + 1 < chain.length) [chain[at + 1], chain[at]] = [chain[at], chain[at + 1]];
            renderRows();
        });
        dialog.done.then(() => { closed = true; schedule.cancel(); composing.clear(); sample = ''; chain.length = 0; });
        renderRows();
        return dialog;
    }
    ED.transform = { build, preview, literal, available, initial, readSample, boundedJSON: bounded, open };
})();
