// EasyDrag condition editors: the condition group (all or any of rows with left side, operator,
// right side and type) and the switch cases built on it. Loads after easydrag-template-field.js
// and before easydrag-fields.js, which re-exports both through ED.fields.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const OPS = ['eq', 'ne', 'contains', 'not_contains', 'starts_with', 'ends_with', 'matches', 'gt', 'gte', 'lt', 'lte', 'before', 'after', 'empty', 'not_empty', 'is_true', 'is_false'];
    const UNARY = new Set(['empty', 'not_empty', 'is_true', 'is_false']);
    const COND_TYPES = ['auto', 'text', 'number', 'date', 'bool'];

    const { withChange } = ED.fieldsShared;
    const { templateField } = ED.fieldsTemplate;

    function core() { return ED.core; }

    // segmented and selectBox stay in easydrag-fields.js, which loads after this file; a condition
    // editor looks them up when it is built.
    function segmented(env, options, value, onChange, aria) { return ED.fields.segmented(env, options, value, onChange, aria); }
    function selectBox(env, options, value, onChange, id, aria) { return ED.fields.selectBox(env, options, value, onChange, id, aria); }

    // objects keeps the object items of a list; a malformed param becomes an empty list.
    function objects(list) { return Array.isArray(list) ? list.filter(x => x && typeof x === 'object' && !Array.isArray(x)) : []; }

    function blankRow() { return { left: '', op: 'eq', right: '', type: 'auto' }; }

    // ── condition group ─────────────────────────────────────────────────────────

    function conditionGroup(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const group = Object.assign({ match: 'all' }, value && typeof value === 'object' && !Array.isArray(value) ? JSON.parse(JSON.stringify(value)) : {});
        group.rows = objects(group.rows);
        if (!group.rows.length) group.rows.push(blankRow());
        const el = c.el('<div class="ed-cond"><div class="ed-cond-head"></div><div class="ed-cond-rows"></div>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-cond-add>' + c.icon('plus') + '<span>' + esc(t('easydrag.ui.cond_add')) + '</span></button>') + '</div>');
        const head = el.querySelector('.ed-cond-head');
        const rows = el.querySelector('.ed-cond-rows');
        const emit = () => onChange(JSON.parse(JSON.stringify(group)));

        head.appendChild(segmented(env, [{ value: 'all', label: t('easydrag.ui.cond_all') }, { value: 'any', label: t('easydrag.ui.cond_any') }], group.match, v => { group.match = v; emit(); },
            { label: t('easydrag.ui.cond_match') }));

        function renderRows() {
            rows.innerHTML = '';
            group.rows.forEach((row, idx) => {
                const r = c.el('<div class="ed-cond-row"><div class="ed-cond-left"></div><div class="ed-cond-op"></div><div class="ed-cond-right"></div><div class="ed-cond-type"></div>' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-cond-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove_row', { n: idx + 1 })) + '">' + c.icon('x') + '</button>') + '</div>');
                const left = templateField(withChange(env, v => { row.left = v; emit(); }), row.left, { placeholder: t('easydrag.ui.cond_left'), label: t('easydrag.ui.cond_left') });
                r.querySelector('.ed-cond-left').appendChild(left.el);
                r.querySelector('.ed-cond-op').appendChild(selectBox(env, OPS.map(op => ({ value: op, label: c.tr(t, 'easydrag.ui.op_' + op, op) })), row.op, v => { row.op = v; emit(); renderRows(); },
                    null, { label: t('easydrag.ui.cond_op') }));
                if (!UNARY.has(row.op)) {
                    const right = templateField(withChange(env, v => { row.right = v; emit(); }), row.right, { placeholder: t('easydrag.ui.cond_right'), label: t('easydrag.ui.cond_right') });
                    r.querySelector('.ed-cond-right').appendChild(right.el);
                }
                r.querySelector('.ed-cond-type').appendChild(selectBox(env, COND_TYPES.map(ty => ({ value: ty, label: c.tr(t, 'easydrag.ui.cond_type_' + ty, ty) })), row.type || 'auto', v => { row.type = v; emit(); },
                    null, { label: t('easydrag.ui.cond_type') }));
                rows.appendChild(r);
            });
        }

        el.addEventListener('click', (event) => {
            if (event.target.closest('[data-ed-cond-add]')) { group.rows.push(blankRow()); emit(); renderRows(); return; }
            const rm = event.target.closest('[data-ed-cond-remove]');
            if (rm) { group.rows.splice(Number(rm.dataset.edCondRemove), 1); if (!group.rows.length) group.rows.push(blankRow()); emit(); renderRows(); }
        });
        renderRows();
        return el;
    }

    function cases(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const list = objects(Array.isArray(value) ? JSON.parse(JSON.stringify(value)) : []);
        const el = c.el('<div class="ed-cases"><div class="ed-cases-list"></div>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-case-add>' + c.icon('plus') + '<span>' + esc(t('easydrag.ui.case_add')) + '</span></button>') + '</div>');
        const host = el.querySelector('.ed-cases-list');
        const emit = () => onChange(JSON.parse(JSON.stringify(list)));
        function render() {
            host.innerHTML = '';
            list.forEach((cs, idx) => {
                const card = c.el('<div class="ed-case"><div class="ed-case-head"><span class="ed-case-port">' + esc(t('easydrag.ui.port_case', { n: idx + 1 })) + '</span>' +
                    '<input class="ed-input ed-case-label" placeholder="' + esc(t('easydrag.ui.case_label')) + '" aria-label="' + esc(t('easydrag.ui.case_label')) + '" value="' + esc(cs.label || '') + '"' + (env.readonly ? ' disabled' : '') + '>' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-case-up="' + idx + '" aria-label="' + esc(t('easydrag.ui.move_up')) + '"' + (idx === 0 ? ' disabled' : '') + '>' + c.icon('chevron-down', 'ed-rot180') + '</button>' +
                        '<button type="button" class="ed-icon-btn" data-ed-case-down="' + idx + '" aria-label="' + esc(t('easydrag.ui.move_down')) + '"' + (idx === list.length - 1 ? ' disabled' : '') + '>' + c.icon('chevron-down') + '</button>' +
                        '<button type="button" class="ed-icon-btn" data-ed-case-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove_row', { n: idx + 1 })) + '">' + c.icon('trash') + '</button>') + '</div></div>');
                card.querySelector('.ed-case-label').addEventListener('change', (event) => { cs.label = event.target.value; emit(); });
                card.appendChild(conditionGroup(env, cs.condition, v => { cs.condition = v; emit(); }));
                host.appendChild(card);
            });
        }
        el.addEventListener('click', (event) => {
            if (event.target.closest('[data-ed-case-add]')) { list.push({ label: '', condition: { match: 'all', rows: [blankRow()] } }); emit(); render(); return; }
            const up = event.target.closest('[data-ed-case-up]');
            if (up) { const i = Number(up.dataset.edCaseUp); list.splice(i - 1, 0, list.splice(i, 1)[0]); emit(); render(); return; }
            const down = event.target.closest('[data-ed-case-down]');
            if (down) { const i = Number(down.dataset.edCaseDown); list.splice(i + 1, 0, list.splice(i, 1)[0]); emit(); render(); return; }
            const rm = event.target.closest('[data-ed-case-remove]');
            if (rm) { list.splice(Number(rm.dataset.edCaseRemove), 1); emit(); render(); }
        });
        render();
        return el;
    }

    ED.fieldsConditions = { conditionGroup, cases, objects, OPS, UNARY };
})();
