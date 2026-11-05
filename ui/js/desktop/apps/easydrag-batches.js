// Bounded serial batches reuse the pinned block editor, ordinary mappings and retained output views.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    function parseItems(raw) {
        const limit = 2 * 1024 * 1024;
        if (typeof raw !== 'string' || raw.length > limit) throw new Error('List limit');
        let bytes = 0;
        for (const char of raw) { const cp = char.codePointAt(0); bytes += cp < 128 ? 1 : cp < 2048 ? 2 : cp < 65536 ? 3 : 4; if (bytes > limit) throw new Error('List limit'); }
        const value = JSON.parse(raw);
        if (!Array.isArray(value)) throw new Error('List required');
        const stack = [[value, 0]]; let count = 0;
        while (stack.length) {
            const [item, depth] = stack.pop();
            if (++count > 200000 || depth > 64 || typeof item === 'number' && !Number.isFinite(item)) throw new Error('List limit');
            if (item && typeof item === 'object') { const children = Object.values(item); if (children.length + stack.length + count > 200000) throw new Error('List limit'); children.forEach(child => stack.push([child, depth + 1])); }
        }
        return value;
    }
    function options(env) {
        const { ed } = env, core = ED.core;
        let node = env.node, closed = false;
        const info = { type: 'batch.options', params: [
            { name: 'items', label: ed.t('easydrag.ui.batch_items'), help: ed.t('easydrag.ui.batch_items_hint'), kind: 'json', required: true, templatable: true, literal: true, parseJSON: parseItems },
            { name: 'max_items', label: ed.t('easydrag.ui.batch_max_items'), help: ed.t('easydrag.ui.batch_limit_hint'), kind: 'number', required: true, min: 1, max: 1000, step: 1 },
            { name: 'on_item_error', label: ed.t('easydrag.ui.batch_on_item_error'), help: ed.t('easydrag.ui.batch_policy_hint'), kind: 'select', required: true, options: [{ value: 'stop', label: ed.t('easydrag.ui.batch_stop') }, { value: 'collect', label: ed.t('easydrag.ui.batch_collect') }] }
        ] };
        const synthetic = () => ({ id: node.id, key: node.key, type: info.type, params: Object.fromEntries(info.params.filter(param => Object.hasOwn(node.params, param.name)).map(param => [param.name, param.name === 'on_item_error' && typeof node.params[param.name] !== 'string' ? ':invalid' : node.params[param.name]])), settings: {} });
        const form = ED.forms.render(Object.assign({}, env, { node: synthetic(), info, onChange: (name, value) => { if (name === 'on_item_error' && value === ':invalid' && typeof node.params.on_item_error !== 'string') return; if (!closed && !ed.readonly && !ed.runView && ed.model.node(node.id) === node) env.onChange(name, value, node.id); } }));
        const el = core.el('<div class="ed-batch-options"></div>'); el.appendChild(form.el);
        const invalid = core.el('<p class="ed-error" role="alert" hidden></p>'); el.appendChild(invalid);
        const fixed = core.el('<div class="ed-callout ed-callout--warn" hidden><p></p><button type="button" class="ed-btn" data-ed-batch-reset-concurrency>' + ed.esc(ed.t('easydrag.ui.apply')) + ' · concurrency: 1</button></div>'); el.appendChild(fixed);
        fixed.querySelector('button').addEventListener('click', () => { if (!closed && !ed.readonly && !ed.runView && ed.model.node(node.id) === node && form.flush()) env.onChange('concurrency', 1, node.id); });
        function preserveInvalid() {
            fixed.hidden = node.params.concurrency === 1;
            fixed.querySelector('p').textContent = ed.t('easydrag.ui.error_flow_param_invalid') + ' concurrency: ' + ED.template.describe(node.params.concurrency, ed.t, core.fmt.number);
            fixed.querySelector('button').disabled = !!(ed.readonly || ed.runView);
            const select = form.el.querySelector('.ed-field[data-param="on_item_error"] select'), value = node.params.on_item_error;
            if (select && value !== undefined && !['stop', 'collect'].includes(value)) { const text = typeof value === 'string' ? value : ':invalid'; if (!Array.from(select.querySelectorAll('option')).some(option => option.value === text)) select.appendChild(core.el('<option value="' + ed.esc(text) + '">' + ed.esc(ed.t('easydrag.ui.error_flow_param_invalid') + ' ' + (typeof value === 'string' ? value.slice(0, 160) : ED.template.describe(value, ed.t, core.fmt.number))) + '</option>')); select.value = text; }
        }
        preserveInvalid();
        return { el, mappingTargets: () => [info.params[0]], insert(name, ref) { if (!closed && !ed.readonly && !ed.runView && ed.model.node(node.id) === node) form.insert(name, ref); }, focus: name => { if (!closed) form.focus(name); }, value: name => node.params[name],
            flush() {
                if (closed || ed.readonly || ed.runView) return true;
                if (!form.flush()) return false;
                const value = node.params.items;
                let valid = Array.isArray(value);
                if (typeof value === 'string') {
                    const parts = ED.template.segments(value);
                    valid = parts.length === 1 && parts[0].expr !== undefined;
                    if (valid) { try { ED.template.parseExpr(parts[0].expr); } catch (_) { valid = false; } }
                }
                invalid.hidden = valid; invalid.textContent = valid ? '' : ed.t('easydrag.ui.batch_items_invalid');
                return valid;
            },
            refresh(next, roots) { node = next; form.refresh(synthetic(), roots, ed.issues); preserveInvalid(); },
            dispose() { closed = true; form.dispose(); el.remove(); }
        };
    }
    function resultMarkup(ed, step, compact) {
        if (!step || step.status !== 'success') return '';
        const output = step.output && typeof step.output === 'object' ? step.output : {};
        const { t, esc } = ed, core = ED.core;
        const counts = ['count', 'succeeded', 'failed'].every(key => Number.isInteger(output[key]) && output[key] >= 0 && output[key] <= 1000) && output.count === output.succeeded + output.failed;
        const hasItems = Array.isArray(output.items), items = hasItems ? output.items : [], shown = items.slice(0, 50);
        const complete = counts && hasItems && items.length === output.count && !step.output_truncated && !Object.hasOwn(output, '_preview') && !Object.hasOwn(output, '_omitted');
        const short = value => { const text = ED.mapping.preview(value); return (typeof text === 'string' ? text : ED.template.describe(value, t, core.fmt.number)).slice(0, 160); };
        const preview = value => value && typeof value === 'object' && !Array.isArray(value) ? Object.keys(value).slice(0, 3).map(key => key.slice(0, 80) + ': ' + short(value[key])).join(' · ') : short(value);
        const summary = counts ? t('easydrag.ui.batch_result_summary', { count: core.fmt.number(output.count), succeeded: core.fmt.number(output.succeeded), failed: core.fmt.number(output.failed) }) : t('easydrag.ui.batch_result_incomplete');
        return '<section class="ed-batch-results" data-ed-batch-summary><h4>' + esc(t('easydrag.ui.batch_result_title')) + '</h4><p>' + esc(summary) + '</p>' +
            (!complete ? '<p class="ed-callout ed-callout--warn">' + esc(t('easydrag.ui.batch_result_incomplete')) + '</p>' : '') +
            (!compact && hasItems ? '<p class="ed-hint">' + esc(t('easydrag.ui.batch_result_visible', { visible: core.fmt.number(shown.length), retained: core.fmt.number(items.length) })) + '</p><ol class="ed-batch-item-results" data-ed-batch-results>' + shown.map(item => {
                const valid = item && Number.isInteger(item.index) && item.index >= 0 && item.index < 1000 && ['success', 'error'].includes(item.status);
                return '<li>' + (valid ? '<strong>' + esc(t('easydrag.ui.batch_item_index') + ' ' + item.index + ' · ' + core.tr(t, 'easydrag.ui.status_' + item.status, item.status)) + '</strong>' + (item.status === 'error' ? '<p>' + esc(core.errorText(t, { body: { code: typeof item.error_code === 'string' ? item.error_code : 'FLOW_NODE_FAILED' } })) + '</p>' : '<code>' + esc(preview(item.output)) + '</code>') : esc(t('easydrag.ui.batch_result_incomplete'))) + '</li>';
            }).join('') + '</ol>' : '') + '</section>';
    }
    ED.batches = { options, resultMarkup, parseItems };
})();
