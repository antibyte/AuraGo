// Compare retained, server-scrubbed run records without replaying any step.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const openDialogs = new WeakMap();

    function compare(left, right) {
        let remaining = 20000, incomplete = false;
        const rows = [];
        function equal(a, b, depth) {
            if (--remaining < 0 || depth > 12) { incomplete = true; return null; }
            const marker = v => v === '[redacted]' || v && typeof v === 'object' && (Object.prototype.hasOwnProperty.call(v, '[redacted]') || typeof v._preview === 'string' && (Object.keys(v).length === 1 || v._omitted || v._truncated === true));
            if (marker(a) || marker(b)) { incomplete = true; return null; }
            if (a === b) return true;
            if (a === null || b === null || typeof a !== typeof b) return false;
            if (typeof a !== 'object') return false;
            if (Array.isArray(a) !== Array.isArray(b)) return false;
            const keys = Object.keys(a), other = Object.keys(b);
            if (keys.length !== other.length) return false;
            let result = true;
            for (const key of keys) {
                if (!Object.prototype.hasOwnProperty.call(b, key)) return false;
                const item = equal(a[key], b[key], depth + 1);
                if (item === false) return false;
                if (item === null) result = null;
            }
            return result;
        }
        function add(node, field, before, after) {
            if (equal(before, after, 0) === true) return;
            if (rows.length < 100) rows.push({ node, field, before, after });
            else incomplete = true;
        }
        const a = left.run || {}, b = right.run || {};
        for (const key of ['status', 'revision', 'trigger_type', 'trigger_data']) add('', key, a[key], b[key]);
        const old = new Map((left.steps || []).slice(0, 500).map(s => [s.node_id, s]));
        const next = new Map((right.steps || []).slice(0, 500).map(s => [s.node_id, s]));
        if ((left.steps || []).length > 500 || (right.steps || []).length > 500) incomplete = true;
        for (const id of new Set([...old.keys(), ...next.keys()])) {
            const before = old.get(id), after = next.get(id);
            if (!before || !after) { add(id, 'status', before && before.status, after && after.status); continue; }
            if (before.params_truncated || before.output_truncated || after.params_truncated || after.output_truncated) incomplete = true;
            for (const field of ['status', 'error_code', 'effect_uncertain', 'ports', 'params', 'output']) {
                add(id, field, before[field], after[field]);
            }
        }
        return { rows, incomplete };
    }

    // Deliberately omit names, messages, documents and payloads from the portable diagnostic.
    function diagnostic(details) {
        const pick = (value, keys) => Object.fromEntries(keys.filter(k => value[k] !== undefined).map(k => [k, value[k]]));
        return {
            format: 'easydrag-diagnostic', schema: 1,
            runs: details.slice(0, 2).map(detail => ({
                run: pick(detail.run || {}, ['id', 'flow_id', 'revision', 'mode', 'status', 'started_at', 'finished_at', 'duration_ms', 'error_code', 'effect_uncertain']),
                steps: (detail.steps || []).slice(0, 500).map(step => pick(step, ['node_id', 'status', 'started_at', 'finished_at', 'duration_ms', 'attempt', 'error_code', 'effect_uncertain', 'params_truncated', 'output_truncated']))
            }))
        };
    }

    function open(ed, records) {
        if (openDialogs.has(ed)) return;
        const core = ED.core, { t, esc } = ed;
        const choices = records.filter(r => ED.runs.isFinal(r.status)).slice(0, 200);
        if (choices.length < 2) return;
        const option = (record, selected) => '<option value="' + esc(record.id) + '"' + (selected ? ' selected' : '') + '>' + esc(core.fmt.dateTime(record.started_at) + ' · ' + core.tr(t, 'easydrag.ui.status_' + record.status, record.status) + ' · ' + record.id) + '</option>';
        let seq = 0, closed = false, current = null;
        const dialog = core.modal(ed.root, {
            title: t('easydrag.ui.compare_runs'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--publish',
            body: '<p class="ed-hint">' + esc(t('easydrag.ui.compare_notice')) + '</p><div class="ed-publish-values">' +
                ['before', 'after'].map((side, i) => '<label>' + esc(t('easydrag.ui.publish_change_' + side)) + '<select class="ed-input" data-ed-compare="' + side + '">' + choices.map((r, n) => option(r, n === (i ? 0 : 1))).join('') + '</select></label>').join('') +
                '</div><p class="ed-error" data-ed-compare-error role="alert" hidden></p><div data-ed-compare-result aria-live="polite"></div>',
            actions: [{ id: 'export', label: t('easydrag.ui.diagnostic_export'), icon: 'download' }, { id: 'close', label: t('easydrag.ui.close') }],
            onAction(id) {
                if (id !== 'export' || !current) return false;
                const url = URL.createObjectURL(new Blob([JSON.stringify(diagnostic(current), null, 2)], { type: 'application/json' }));
                const anchor = document.createElement('a');
                anchor.href = url; anchor.download = 'easydrag-diagnostic.json';
                document.body.appendChild(anchor); anchor.click(); anchor.remove();
                setTimeout(() => URL.revokeObjectURL(url), 1000);
                return false;
            }
        });
        openDialogs.set(ed, dialog);
        const exportButton = dialog.el.querySelector('[data-ed-action="export"]');
        dialog.done.then(() => { closed = true; ++seq; current = null; openDialogs.delete(ed); });
        let previewShortened = false;
        const value = (data, field) => {
            if (data === undefined) return '—';
            if (field === 'status' && typeof data === 'string') return core.tr(t, 'easydrag.ui.status_' + data, data);
            if (field === 'error_code' && data) return core.stepErrorText(t, { error_code: data });
            const text = JSON.stringify(data, null, 2);
            if (text.length > 1200) { previewShortened = true; return text.slice(0, 1200) + '…'; }
            return text;
        };
        async function load() {
            const mine = ++seq;
            current = null; exportButton.disabled = true;
            const result = dialog.body.querySelector('[data-ed-compare-result]');
            const error = dialog.body.querySelector('[data-ed-compare-error]');
            error.hidden = true;
            result.textContent = t('easydrag.ui.loading');
            const ids = ['before', 'after'].map(side => dialog.body.querySelector('[data-ed-compare="' + side + '"]').value);
            try {
                const details = await Promise.all(ids.map(id => ed.api.run(id, false)));
                if (closed || mine !== seq) return;
                if (details.some((detail, i) => !detail.run || detail.run.id !== ids[i] || detail.run.flow_id !== ed.flow.id || !ED.runs.isFinal(detail.run.status))) throw Object.assign(new Error(), { code: 'FLOW_RUN_NOT_FOUND' });
                current = details;
                exportButton.disabled = false;
                const changes = compare(details[0], details[1]);
                const names = new Map(details.flatMap(detail => detail.steps || []).map(step => [step.node_id, step.node_key || step.node_id]));
                const labels = {
                    status: t('easydrag.ui.compare_status'), revision: t('easydrag.ui.compare_revision'),
                    trigger_type: t('easydrag.ui.root_trigger'), trigger_data: t('easydrag.ui.detail_input'),
                    params: t('easydrag.ui.detail_input'), output: t('easydrag.ui.detail_output'),
                    error_code: t('easydrag.ui.compare_error'), ports: t('easydrag.ui.compare_branches'),
                    effect_uncertain: t('easydrag.ui.step_effect_uncertain')
                };
                previewShortened = false;
                const markup = changes.rows.map(row => '<article class="ed-publish-change"><h4 title="' + esc(row.node) + '">' + esc((row.node ? names.get(row.node) + ' · ' : '') + (labels[row.field] || row.field)) + '</h4><div class="ed-publish-values"><pre>' + esc(value(row.before, row.field)) + '</pre><pre>' + esc(value(row.after, row.field)) + '</pre></div></article>').join('');
                result.innerHTML = '<section class="ed-publish-changes">' + markup + '</section>' +
                    (changes.incomplete || previewShortened ? '<p class="ed-hint">' + esc(t('easydrag.ui.compare_incomplete')) + '</p>' : !changes.rows.length ? '<p class="ed-hint">' + esc(t('easydrag.ui.diff_none')) + '</p>' : '');
            } catch (err) {
                if (closed || mine !== seq) return;
                current = null; exportButton.disabled = true;
                result.textContent = ''; error.hidden = false; error.textContent = core.errorText(t, err);
            }
        }
        dialog.body.querySelectorAll('[data-ed-compare]').forEach(select => select.addEventListener('change', load));
        load();
    }
    function close(ed) { const dialog = openDialogs.get(ed); if (dialog) dialog.close(); }
    ED.compare = { compare, diagnostic, open, close };
})();
