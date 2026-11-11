// EasyDrag parameter editors: the simple controls (segmented, switch, select) and the complex
// editors (field lists, decisions, key-value, JSON, tags, file, secrets, date and time).
// ED.fields merges them with the template field (easydrag-template-field.js) and the condition
// group and cases (easydrag-fields-conditions.js), which load first. Every editor calls
// env.change(value) on edits.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // SECRET_MAX_BYTES is the server's limit for a flow secret value (UTF-8 bytes).
    const SECRET_MAX_BYTES = 4096;
    // The template field, the shared helpers and the condition editors load first; ED.fields below merges them.
    const { nameAttrs, withChange, composing, nextFieldId } = ED.fieldsShared;
    const { templateField, chipsMarkup, sampleAt } = ED.fieldsTemplate;
    const { conditionGroup, cases, objects, OPS, UNARY } = ED.fieldsConditions;

    function core() { return ED.core; }

    // ── simple controls ─────────────────────────────────────────────────────────

    // segmented, selectBox: aria ({labelledBy} or {label}) names the control.
    function segmented(env, options, value, onChange, aria) {
        const { esc } = env;
        const el = core().el('<div class="ed-seg" role="radiogroup"' + nameAttrs(esc, aria) + '>' + options.map(o =>
            '<button type="button" role="radio" aria-checked="' + (String(o.value) === String(value)) + '" data-value="' + esc(o.value) + '"' + (env.readonly ? ' disabled' : '') + '>' + esc(o.label) + '</button>').join('') + '</div>');
        el.addEventListener('click', (event) => {
            const b = event.target.closest('button');
            if (!b) return;
            el.querySelectorAll('button').forEach(x => x.setAttribute('aria-checked', String(x === b)));
            onChange(b.dataset.value);
        });
        return el;
    }

    function toggle(env, value, onChange, label) {
        const { esc } = env;
        const el = core().el('<button type="button" class="ed-switch" role="switch" aria-checked="' + (!!value) + '" aria-label="' + esc(label || '') + '"' + (env.readonly ? ' disabled' : '') + '><span></span></button>');
        el.addEventListener('click', () => {
            const next = el.getAttribute('aria-checked') !== 'true';
            el.setAttribute('aria-checked', String(next));
            onChange(next);
        });
        return el;
    }

    function selectBox(env, options, value, onChange, id, aria) {
        const { esc } = env;
        const el = core().el('<select class="ed-input"' + (id ? ' id="' + esc(id) + '"' : '') + nameAttrs(esc, aria) + (env.readonly ? ' disabled' : '') + '>' +
            options.map(o => '<option value="' + esc(o.value) + '"' + (String(o.value) === String(value == null ? '' : value) ? ' selected' : '') + '>' + esc(o.label) + '</option>').join('') + '</select>');
        el.addEventListener('change', () => onChange(el.value));
        return el;
    }

    // fieldList edits [{name, type, value|description}] using the param's sub-field specs.
    function fieldList(env, spec, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const list = objects(Array.isArray(value) ? JSON.parse(JSON.stringify(value)) : []);
        const subs = spec.fields && spec.fields.length ? spec.fields :[{ name: 'name', kind: 'text', label: t('easydrag.ui.field_name') }, { name: 'value', kind: 'text', label: t('easydrag.ui.field_value'), templatable: true }];
        const el = c.el('<div class="ed-fieldlist"><div class="ed-fieldlist-head">' + subs.map(s => '<span>' + esc(s.label) + '</span>').join('') + '<span></span></div><div class="ed-fieldlist-rows"></div>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-fl-add>' + c.icon('plus') + '<span>' + esc(t('easydrag.ui.fields_add')) + '</span></button>') + '</div>');
        el.style.setProperty('--ed-fl-cols', subs.length);
        const rows = el.querySelector('.ed-fieldlist-rows');
        const emit = () => onChange(JSON.parse(JSON.stringify(list)));
        function render() {
            rows.innerHTML = '';
            list.forEach((item, idx) => {
                const row = c.el('<div class="ed-fieldlist-row"></div>');
                subs.forEach(s => {
                    const cell = c.el('<div class="ed-fieldlist-cell"></div>');
                    const set = v => { item[s.name] = v; emit(); };
                    if (s.options && s.options.length) cell.appendChild(selectBox(env, s.options, item[s.name] == null ? s.default : item[s.name], set, null, { label: s.label }));
                    else if (s.templatable) cell.appendChild(templateField(withChange(env, set), item[s.name], { placeholder: s.label, label: s.label }).el);
                    else {
                        const input = c.el('<input class="ed-input" value="' + esc(item[s.name] == null ? '' : item[s.name]) + '" placeholder="' + esc(s.label) + '" aria-label="' + esc(s.label) + '"' + (env.readonly ? ' disabled' : '') + '>');
                        input.addEventListener('change', () => set(input.value));
                        cell.appendChild(input);
                    }
                    row.appendChild(cell);
                });
                if (!env.readonly) row.appendChild(c.el('<button type="button" class="ed-icon-btn" data-ed-fl-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove_row', { n: idx + 1 })) + '">' + c.icon('x') + '</button>'));
                rows.appendChild(row);
            });
        }
        el.addEventListener('click', (event) => {
            if (event.target.closest('[data-ed-fl-add]')) {
                const item = {};
                subs.forEach(s => { if (s.default !== undefined) item[s.name] = s.default; });
                list.push(item); emit(); render(); return;
            }
            const rm = event.target.closest('[data-ed-fl-remove]');
            if (rm) { list.splice(Number(rm.dataset.edFlRemove), 1); emit(); render(); }
        });
        render();
        return el;
    }

    function nextStableID(prefix, used) {
        for (let index = 1; index < 10000; index++) {
            const id = prefix + '_' + index;
            if (!used.has(id)) { used.add(id); return id; }
        }
        return prefix + '_' + Date.now().toString(36);
    }

    // decisionQuestions edits an ordered question list. IDs are persisted independently
    // from row order so ports remain attached to the same choice when options move.
    function decisionQuestions(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const list = objects(Array.isArray(value) ? JSON.parse(JSON.stringify(value)) : []);
        const mode = env.node && env.node.params && env.node.params.mode === 'evaluate' ? 'evaluate' : 'route';
        const typeOptions = [
            { value: 'choice', label: t('easydrag.decision.type_choice') },
            { value: 'noul', label: t('easydrag.decision.type_noul') },
            { value: 'score', label: t('easydrag.decision.type_score') }
        ];
        const el = c.el('<div class="ed-decision-questions"><div class="ed-decision-questions-list"></div>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-decision-add-question>' + c.icon('plus') + '<span>' + esc(t('easydrag.decision.add_question')) + '</span></button>') + '</div>');
        const host = el.querySelector('.ed-decision-questions-list');
        const emit = () => onChange(JSON.parse(JSON.stringify(list)));

        function render() {
            host.innerHTML = '';
            list.forEach((question, qi) => {
                const questionID = String(question.id || '');
                const type = ['choice', 'noul', 'score'].includes(question.type) ? question.type : 'choice';
                const routeExtra = mode === 'route' && qi > 0;
                const card = c.el('<section class="ed-decision-card" data-question-index="' + qi + '">' +
                    '<header class="ed-decision-card-head"><strong>' + esc(t('easydrag.decision.question_number', { n: qi + 1 })) + '</strong><div class="ed-decision-actions">' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-decision-question-up="' + qi + '" aria-label="' + esc(t('easydrag.ui.move_up')) + '"' + (qi === 0 ? ' disabled' : '') + '>' + c.icon('chevron-down', 'ed-rot180') + '</button>' +
                        '<button type="button" class="ed-icon-btn" data-ed-decision-question-down="' + qi + '" aria-label="' + esc(t('easydrag.ui.move_down')) + '"' + (qi === list.length - 1 ? ' disabled' : '') + '>' + c.icon('chevron-down') + '</button>' +
                        '<button type="button" class="ed-icon-btn" data-ed-decision-question-remove="' + qi + '" aria-label="' + esc(t('easydrag.ui.remove_row', { n: qi + 1 })) + '">' + c.icon('trash') + '</button>') +
                    '</div></header><div class="ed-decision-grid"><label>' + esc(t('easydrag.decision.id')) + '<input class="ed-input ed-code" data-ed-decision-question-id value="' + esc(questionID) + '"' + (env.readonly ? ' disabled' : '') + '></label><label>' + esc(t('easydrag.decision.question_type')) + '<select class="ed-input" data-ed-decision-type' + (env.readonly ? ' disabled' : '') + '>' +
                    typeOptions.map(option => '<option value="' + option.value + '"' + (type === option.value ? ' selected' : '') + '>' + esc(option.label) + '</option>').join('') + '</select></label></div>' +
                    '<div class="ed-decision-instructions"></div><div class="ed-decision-options"></div>' +
                    (routeExtra ? '<p class="ed-hint ed-decision-warning">' + esc(t('easydrag.decision.route_one_question')) + '</p>' : '') + '</section>');
                card.querySelector('[data-ed-decision-question-id]').addEventListener('change', event => { question.id = event.target.value; emit(); });
                card.querySelector('[data-ed-decision-type]').addEventListener('change', event => {
                    question.type = event.target.value;
                    if (question.type === 'noul') question.options = [
                        { id: 'no', label: t('easydrag.decision.no') }, { id: 'yes', label: t('easydrag.decision.yes') }
                    ];
                    else if (type === 'noul' || !Array.isArray(question.options) || question.options.length < 2) {
                        question.options = [{ id: 'option_a', label: '' }, { id: 'option_b', label: '' }];
                    } else if (question.type === 'choice') {
                        // Score-only values are invalid for choice questions; keep the option IDs,
                        // labels, and their stable output ports when switching back to routing.
                        question.options.forEach(option => {
                            if (option && typeof option === 'object' && !Array.isArray(option)) delete option.value;
                        });
                    }
                    emit(); render();
                });
                card.querySelector('.ed-decision-instructions').appendChild(templateField(withChange(env, next => { question.instructions = next; emit(); }), question.instructions, {
                    multiline: true, placeholder: t('easydrag.decision.instructions'), label: t('easydrag.decision.instructions')
                }).el);
                const optionHost = card.querySelector('.ed-decision-options');
                const options = objects(Array.isArray(question.options) ? question.options : []);
                const noAction = env.readonly;
                if (type === 'noul') {
                    // Noul's two branches are semantic and fixed; only their display labels can change.
                    const byID = new Map(options.map(option => [option.id, option]));
                    ['no', 'yes'].forEach((id, index) => {
                        const option = byID.get(id) || { id, label: t('easydrag.decision.' + id) };
                        const row = c.el('<div class="ed-decision-option ed-decision-option--noul"><code>' + id + '</code><input class="ed-input" data-ed-decision-label placeholder="' + esc(t('easydrag.decision.option_label')) + '" aria-label="' + esc(t('easydrag.decision.' + id)) + '" value="' + esc(option.label || '') + '"' + (noAction ? ' disabled' : '') + '></div>');
                        row.querySelector('input').addEventListener('change', event => {
                            const next = ['no', 'yes'].map(key => byID.get(key) || { id: key, label: t('easydrag.decision.' + key) });
                            next[index].label = event.target.value;
                            question.options = next; emit();
                        });
                        optionHost.appendChild(row);
                    });
                } else {
                    const used = new Set(options.map(option => String(option.id || '')));
                    options.forEach((option, oi) => {
                        const row = c.el('<div class="ed-decision-option' + (type === 'score' ? ' ed-decision-option--score' : '') + '"><input class="ed-input ed-code" data-ed-decision-option-id value="' + esc(option.id || '') + '" aria-label="' + esc(t('easydrag.decision.option_id')) + '"' + (noAction ? ' disabled' : '') + '>' +
                            '<input class="ed-input" data-ed-decision-label placeholder="' + esc(t('easydrag.decision.option_label')) + '" aria-label="' + esc(t('easydrag.decision.option_label')) + '" value="' + esc(option.label || '') + '"' + (noAction ? ' disabled' : '') + '>' +
                            (type === 'score' ? '<input class="ed-input ed-decision-score" data-ed-decision-score type="number" step="any" placeholder="' + esc(t('easydrag.decision.score_default', { n: oi })) + '" aria-label="' + esc(t('easydrag.decision.score_value')) + '" value="' + esc(option.value == null ? '' : option.value) + '"' + (noAction ? ' disabled' : '') + '>' : '') +
                            '<div class="ed-decision-actions">' + (noAction ? '' : '<button type="button" class="ed-icon-btn" data-ed-decision-option-up="' + oi + '" aria-label="' + esc(t('easydrag.ui.move_up')) + '"' + (oi === 0 ? ' disabled' : '') + '>' + c.icon('chevron-down', 'ed-rot180') + '</button><button type="button" class="ed-icon-btn" data-ed-decision-option-down="' + oi + '" aria-label="' + esc(t('easydrag.ui.move_down')) + '"' + (oi === options.length - 1 ? ' disabled' : '') + '>' + c.icon('chevron-down') + '</button><button type="button" class="ed-icon-btn" data-ed-decision-option-remove="' + oi + '" aria-label="' + esc(t('easydrag.ui.remove_row', { n: oi + 1 })) + '">' + c.icon('trash') + '</button>') + '</div></div>');
                        row.querySelector('[data-ed-decision-option-id]').addEventListener('change', event => { option.id = event.target.value; emit(); });
                        row.querySelector('[data-ed-decision-label]').addEventListener('change', event => { option.label = event.target.value; emit(); });
                        const score = row.querySelector('[data-ed-decision-score]');
                        if (score) score.addEventListener('change', event => { option.value = event.target.value === '' ? undefined : Number(event.target.value); emit(); });
                        optionHost.appendChild(row);
                    });
                    if (!env.readonly) {
                        const add = c.el('<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-decision-add-option' + (options.length >= 62 ? ' disabled' : '') + '>' + c.icon('plus') + '<span>' + esc(t('easydrag.decision.add_option')) + '</span></button>');
                        optionHost.appendChild(add);
                    }
                }
                host.appendChild(card);
            });
            const addQuestion = el.querySelector('[data-ed-decision-add-question]');
            if (addQuestion) addQuestion.disabled = list.length >= 16 || (mode === 'route' && list.length >= 1);
        }
        el.addEventListener('click', event => {
            let button = event.target.closest('[data-ed-decision-add-question]');
            if (button && !button.disabled) {
                const used = new Set(list.map(question => String(question.id || '')));
                const id = nextStableID('question', used);
                list.push({ id, type: 'choice', instructions: '', options: [{ id: 'option_a', label: '' }, { id: 'option_b', label: '' }] }); emit(); render(); return;
            }
            button = event.target.closest('[data-ed-decision-question-up],[data-ed-decision-question-down]');
            if (button) {
                const index = Number(button.dataset.edDecisionQuestionUp || button.dataset.edDecisionQuestionDown);
                const target = button.getAttribute('data-ed-decision-question-up') !== null ? index - 1 : index + 1;
                list.splice(target, 0, list.splice(index, 1)[0]); emit(); render(); return;
            }
            button = event.target.closest('[data-ed-decision-question-remove]');
            if (button) { list.splice(Number(button.dataset.edDecisionQuestionRemove), 1); emit(); render(); return; }
            button = event.target.closest('[data-ed-decision-add-option]');
            if (button && !button.disabled) {
                const card = button.closest('[data-question-index]');
                const question = list[Number(card.dataset.questionIndex)];
                const options = objects(question.options);
                const used = new Set(options.map(option => String(option.id || '')));
                options.push({ id: nextStableID('option', used), label: '' }); question.options = options; emit(); render(); return;
            }
            button = event.target.closest('[data-ed-decision-option-up],[data-ed-decision-option-down]');
            if (button) {
                const card = button.closest('[data-question-index]');
                const question = list[Number(card.dataset.questionIndex)];
                const options = objects(question.options);
                const index = Number(button.dataset.edDecisionOptionUp || button.dataset.edDecisionOptionDown);
                const target = button.getAttribute('data-ed-decision-option-up') !== null ? index - 1 : index + 1;
                options.splice(target, 0, options.splice(index, 1)[0]); question.options = options; emit(); render(); return;
            }
            button = event.target.closest('[data-ed-decision-option-remove]');
            if (button) {
                const card = button.closest('[data-question-index]');
                const question = list[Number(card.dataset.questionIndex)];
                const options = objects(question.options); options.splice(Number(button.dataset.edDecisionOptionRemove), 1); question.options = options; emit(); render();
            }
        });
        render();
        return el;
    }

    function decisionImages(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const list = Array.isArray(value) ? value.map(item => typeof item === 'string' ? item : '') : [];
        const el = c.el('<div class="ed-decision-images"><p class="ed-hint">' + esc(t('easydrag.decision.image_preparation')) + '</p><div class="ed-decision-images-list"></div>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-decision-add-image>' + c.icon('plus') + '<span>' + esc(t('easydrag.decision.add_image')) + '</span></button>') + '</div>');
        const host = el.querySelector('.ed-decision-images-list');
        const emit = () => onChange(list.slice());
        function render() {
            host.innerHTML = '';
            list.forEach((ref, index) => {
                const row = c.el('<div class="ed-decision-image"><div class="ed-decision-image-ref"></div><div class="ed-decision-actions">' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-decision-image-up="' + index + '" aria-label="' + esc(t('easydrag.ui.move_up')) + '"' + (index === 0 ? ' disabled' : '') + '>' + c.icon('chevron-down', 'ed-rot180') + '</button><button type="button" class="ed-icon-btn" data-ed-decision-image-down="' + index + '" aria-label="' + esc(t('easydrag.ui.move_down')) + '"' + (index === list.length - 1 ? ' disabled' : '') + '>' + c.icon('chevron-down') + '</button><button type="button" class="ed-icon-btn" data-ed-decision-image-remove="' + index + '" aria-label="' + esc(t('easydrag.ui.remove_row', { n: index + 1 })) + '">' + c.icon('trash') + '</button>') + '</div></div>');
                row.querySelector('.ed-decision-image-ref').appendChild(templateField(withChange(env, next => { list[index] = next; emit(); }), ref, {
                    placeholder: t('easydrag.decision.image_reference'), label: t('easydrag.decision.image_reference')
                }).el);
                host.appendChild(row);
            });
            const add = el.querySelector('[data-ed-decision-add-image]');
            if (add) add.disabled = list.length >= 8;
        }
        el.addEventListener('click', event => {
            const add = event.target.closest('[data-ed-decision-add-image]');
            if (add && !add.disabled) { list.push(''); emit(); render(); return; }
            const remove = event.target.closest('[data-ed-decision-image-remove]');
            if (remove) { list.splice(Number(remove.dataset.edDecisionImageRemove), 1); emit(); render(); return; }
            const move = event.target.closest('[data-ed-decision-image-up],[data-ed-decision-image-down]');
            if (move) {
                const index = Number(move.dataset.edDecisionImageUp || move.dataset.edDecisionImageDown);
                const target = move.getAttribute('data-ed-decision-image-up') !== null ? index - 1 : index + 1;
                list.splice(target, 0, list.splice(index, 1)[0]); emit(); render();
            }
        });
        render();
        return el;
    }

    // keyValue edits an object of keys and values (HTTP headers, for example). Keys are compared
    // trimmed. While two rows share a key nothing is emitted (an object cannot hold both, and the
    // later one would silently replace the earlier): those rows are marked aria-invalid and point
    // at one message through aria-describedby. A value the editor did not change keeps its type, so
    // a number or a flag from an import stays one (an object is cloned); an edited value is text.
    function keyValue(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const source = value && typeof value === 'object' && !Array.isArray(value) ? value : {};
        const shown = v => v == null ? '' : typeof v === 'object' ? JSON.stringify(v) : String(v);
        const pairs = Object.entries(source).map(([k, v]) => ({ k, v: shown(v), raw: v, edited: false }));
        const errorId = 'ed-kv-error-' + nextFieldId();
        const el = c.el('<div class="ed-kv"><div class="ed-kv-rows"></div>' +
            '<p class="ed-json-error" id="' + errorId + '" aria-live="polite" data-ed-kv-error></p>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-kv-add>' + c.icon('plus') + '<span>' + esc(t('easydrag.ui.kv_add')) + '</span></button>') + '</div>');
        const rows = el.querySelector('.ed-kv-rows');
        const error = el.querySelector('[data-ed-kv-error]');
        error.hidden = true;
        const valueOf = p => {
            if (p.edited || typeof p.raw === 'string') return p.v;
            return p.raw && typeof p.raw === 'object' ? JSON.parse(JSON.stringify(p.raw)) : p.raw;
        };
        // markDuplicates marks the rows whose trimmed key another row has too and reports whether any does.
        const markDuplicates = () => {
            const counts = new Map();
            pairs.forEach(p => { const k = p.k.trim(); if (k) counts.set(k, (counts.get(k) || 0) + 1); });
            let any = false;
            rows.querySelectorAll('[data-ed-kv-key]').forEach((inp, i) => {
                const k = (pairs[i] && pairs[i].k || '').trim();
                const dup = Boolean(k && (counts.get(k) || 0) > 1);
                any = any || dup;
                inp.classList.toggle('is-invalid', dup);
                if (dup) { inp.setAttribute('aria-invalid', 'true'); inp.setAttribute('aria-describedby', errorId); }
                else { inp.removeAttribute('aria-invalid'); inp.removeAttribute('aria-describedby'); }
            });
            error.textContent = any ? t('easydrag.ui.kv_duplicate_key') : '';
            error.hidden = !any;
            return any;
        };
        const emit = () => {
            if (markDuplicates()) return;
            const out = {};
            pairs.forEach(p => { if (p.k.trim()) out[p.k.trim()] = valueOf(p); });
            onChange(out);
        };
        function render() {
            rows.innerHTML = '';
            pairs.forEach((p, idx) => {
                const row = c.el('<div class="ed-kv-row"><input class="ed-input" data-ed-kv-key placeholder="' + esc(t('easydrag.ui.kv_key')) + '" aria-label="' + esc(t('easydrag.ui.kv_key')) + '" value="' + esc(p.k) + '"' + (env.readonly ? ' disabled' : '') + '><div class="ed-kv-value"></div>' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-kv-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove_row', { n: idx + 1 })) + '">' + c.icon('x') + '</button>') + '</div>');
                const keyInp = row.querySelector('input');
                const onKeyEdit = (event) => { p.k = event.target.value; emit(); };
                keyInp.addEventListener('input', onKeyEdit);
                keyInp.addEventListener('change', onKeyEdit);
                row.querySelector('.ed-kv-value').appendChild(templateField(withChange(env, v => { p.v = v; p.edited = true; emit(); }), p.v, { placeholder: t('easydrag.ui.kv_value'), label: t('easydrag.ui.kv_value') }).el);
                rows.appendChild(row);
            });
            markDuplicates();
        }
        el.addEventListener('click', (event) => {
            if (event.target.closest('[data-ed-kv-add]')) { pairs.push({ k: '', v: '', raw: '', edited: false }); render(); return; }
            const rm = event.target.closest('[data-ed-kv-remove]');
            if (rm) { pairs.splice(Number(rm.dataset.edKvRemove), 1); render(); emit(); }
        });
        render();
        return el;
    }

    function jsonEditor(env, value, onChange, id) {
        const c = core();
        const { t, esc } = env;
        const text = value === undefined ? '' : JSON.stringify(value, null, 2);
        const errorId = (id || 'ed-json-' + nextFieldId()) + '-error';
        const el = c.el('<div class="ed-json"><textarea class="ed-input ed-code" rows="6" spellcheck="false"' + (id ? ' id="' + esc(id) + '"' : '') + (env.readonly ? ' disabled' : '') + '></textarea>' +
            '<div class="ed-json-error" id="' + esc(errorId) + '" role="alert" hidden></div></div>');
        const area = el.querySelector('textarea');
        area.value = text;
        const error = el.querySelector('.ed-json-error');
        let committed = area.value;
        // showError shows the message as an alert and links it to the field while the text is no JSON (U5).
        function showError(on) {
            error.hidden = !on;
            if (on) {
                // Written once while the error shows: a rewrite would make a screen reader read it on every keystroke.
                const text = t('easydrag.ui.json_invalid');
                if (error.textContent !== text) error.textContent = text;
                area.setAttribute('aria-invalid', 'true');
                area.setAttribute('aria-describedby', errorId);
            } else if (area.getAttribute('aria-describedby') === errorId) {
                area.removeAttribute('aria-invalid');
                area.removeAttribute('aria-describedby');
            }
        }
        function commit() {
            if (area.edComposing) return false;
            if (area.value === committed && error.hidden) return true;
            const raw = env.parseJSON ? area.value : area.value.trim();
            if (!raw) { showError(false); committed = area.value; onChange(undefined); return true; }
            try {
                const parsed = env.parseJSON ? env.parseJSON(raw) : JSON.parse(raw);
                showError(false);
                committed = area.value;
                onChange(parsed);
                return true;
            } catch (err) {
                showError(true);
                return false;
            }
        }
        // Change and blur announce a standing error afresh.
        const announceAgain = () => {
            if (error.hidden) return;
            const text = error.textContent;
            error.textContent = '';
            error.textContent = text;
        };
        area.addEventListener('input', () => { commit(); });
        area.addEventListener('change', announceAgain);
        area.addEventListener('blur', announceAgain);
        area.edCommit = commit;
        return el;
    }

    function tags(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const list = Array.isArray(value) ? value.filter(v => v != null && typeof v !== 'object').map(String) : [];
        const el = c.el('<div class="ed-tags"><div class="ed-tags-list"></div><input class="ed-input ed-tags-input" placeholder="' + esc(t('easydrag.ui.tags_add')) + '" aria-label="' + esc(t('easydrag.ui.tags_add')) + '" enterkeyhint="done"' + (env.readonly ? ' disabled' : '') + '></div>');
        const host = el.querySelector('.ed-tags-list');
        const input = el.querySelector('input');
        function render() {
            host.innerHTML = list.map((v, i) => '<span class="ed-tag">' + esc(v) + (env.readonly ? '' : '<button type="button" data-ed-tag-remove="' + i + '" aria-label="' + esc(t('easydrag.ui.remove_tag', { name: v })) + '">' + c.icon('x') + '</button>') + '</span>').join('');
        }
        input.addEventListener('keydown', (event) => {
            if (composing(event)) return;
            if ((event.key === 'Enter' || event.key === ',') && input.value.trim()) {
                event.preventDefault();
                list.push(input.value.trim());
                input.value = '';
                onChange(list.slice());
                render();
            }
        });
        el.addEventListener('click', (event) => {
            const rm = event.target.closest('[data-ed-tag-remove]');
            if (rm) { list.splice(Number(rm.dataset.edTagRemove), 1); onChange(list.slice()); render(); }
        });
        render();
        return el;
    }

    function dateTime(env, value, onChange, id) {
        const c = core();
        const { esc } = env;
        const v = String(value || '').replace(' ', 'T').slice(0, 16);
        const el = c.el('<input type="datetime-local" class="ed-input"' + (id ? ' id="' + esc(id) + '"' : '') + ' value="' + esc(v) + '"' + (env.readonly ? ' disabled' : '') + '>');
        el.addEventListener('change', () => onChange(el.value ? el.value.replace('T', ' ') : ''));
        return el;
    }

    // fileField accepts a file reference ({{pdf.file}}) or a path in the workspace or the
    // documents folder; the server checks the path before anything leaves AuraGo.
    function fileField(env, value, onChange, id, aria) {
        const text = typeof value === 'object' && value ? (value.path || '') : value;
        const field = templateField(withChange(env, onChange), text, Object.assign({ id, placeholder: env.t('easydrag.ui.file_placeholder') }, aria));
        field.el.classList.add('ed-file');
        return field.el;
    }

    // utf8Bytes counts the UTF-8 bytes of s, the unit of the server's secret limit (a lone
    // surrogate arrives there as U+FFFD, 3 bytes).
    function utf8Bytes(s) {
        let n = 0;
        for (const ch of String(s)) { const cp = ch.codePointAt(0); n += cp < 0x80 ? 1 : cp < 0x800 ? 2 : cp < 0x10000 ? 3 : 4; }
        return n;
    }

    // secretNames lists the flow secret names. The pickers of one form share the answer through
    // env.secretCache; a failed request is not kept, so the next picker asks again.
    function secretNames(env) {
        const cache = env.secretCache || {};
        if (!cache.names) {
            const names = Promise.resolve().then(() => env.api.secrets())
                .then(res => (res && Array.isArray(res.secrets) ? res.secrets : []).filter(n => typeof n === 'string'));
            cache.names = names;
            names.catch(() => { if (cache.names === names) cache.names = null; });
        }
        return cache.names;
    }

    // holds keeps the timers of each held button, so a newer hold of that button and its
    // dialog's close end them; watched holds the buttons whose dialog close is awaited.
    const holds = new WeakMap();
    const watched = new WeakSet();

    function endHold(button) {
        const ids = holds.get(button);
        if (ids) ids.forEach(clearTimeout);
        holds.delete(button);
    }

    // holdAction keeps a dialog button disabled for seconds (a 429 with Retry-After). The dialog
    // enables its buttons once the action returns, so the hold starts after that. Holds are kept
    // per button: a newer hold of the same button replaces the older one, and every hold's timers
    // (up to an hour) end when its dialog closes.
    function holdAction(dialog, id, seconds) {
        const button = dialog.el.querySelector('[data-ed-action="' + id + '"]');
        if (!button || !(seconds > 0)) return;
        if (!watched.has(button) && dialog.done && typeof dialog.done.then === 'function') {
            watched.add(button);
            dialog.done.then(() => endHold(button));
        }
        endHold(button);
        holds.set(button, [
            setTimeout(() => { button.disabled = true; }, 0),
            setTimeout(() => { button.disabled = false; holds.delete(button); }, Math.min(seconds, 3600) * 1000)
        ]);
    }

    // namesText lists names in the page's language: up to three as a list with "and" (Intl.ListFormat),
    // more as three names and "and N more". The three are joined with the language's own list
    // separator (", ", "、"); without Intl.ListFormat with ", ".
    function namesText(t, names) {
        let list = null;
        try { list = new Intl.ListFormat(core().lang(), { style: 'long', type: 'conjunction' }); } catch (err) { list = null; }
        if (names.length <= 3) return list ? list.format(names) : names.join(', ');
        let sep = ', ';
        if (list) {
            const literal = list.formatToParts(['a', 'b', 'c']).find(p => p.type === 'literal');
            if (literal) sep = literal.value;
        }
        return t('easydrag.ui.names_more', { names: names.slice(0, 3).join(sep), count: names.length - 3 });
    }

    // secretRef picks a flow secret (vault entry easydrag_<name>), creates a new one or deletes the
    // chosen one. A name that exists is replaced, and a secret deleted, only after a confirmation.
    function secretRef(env, value, onChange, id) {
        const c = core();
        const { t, esc } = env;
        const el = c.el('<div class="ed-secret"><select class="ed-input"' + (id ? ' id="' + esc(id) + '"' : '') + (env.readonly ? ' disabled' : '') + '></select>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-secret-new>' + c.icon('key') + '<span>' + esc(t('easydrag.ui.secret_new')) + '</span></button>' +
                '<button type="button" class="ed-icon-btn" data-ed-secret-delete disabled title="' + esc(t('easydrag.ui.secret_delete')) + '" aria-label="' + esc(t('easydrag.ui.secret_delete')) + '">' + c.icon('trash') + '</button>') +
            '<p class="ed-hint ed-secret-unavailable" hidden>' + esc(t('easydrag.ui.secrets_unavailable')) + '</p></div>');
        const select = el.querySelector('select');
        const unavailable = el.querySelector('.ed-secret-unavailable');
        const deleteBtn = el.querySelector('[data-ed-secret-delete]');
        // Delete works on the chosen secret: without one there is nothing to delete.
        const syncDelete = () => { if (deleteBtn) deleteBtn.disabled = !select.value; };
        async function load(selected) {
            let names = [];
            let missing = false;
            try {
                names = (await secretNames(env)).slice();
                unavailable.hidden = true;
                select.removeAttribute('data-error');
                // A chosen secret the Vault no longer has: runs fail until it is set again.
                missing = !!selected && !names.includes(selected);
                if (missing) select.dataset.error = t('easydrag.ui.error_flow_secret_unavailable');
            } catch (err) {
                // Like a failed option list: the reason on the select, a visible hint under it.
                unavailable.hidden = false;
                select.dataset.error = c.errorText(t, err);
            }
            if (selected && !names.includes(selected)) names.push(selected);
            select.innerHTML = '<option value="">' + esc(t('easydrag.ui.secret_none')) + '</option>' + names.map(n => '<option value="' + esc(n) + '"' + (n === selected ? ' selected' : '') + '>' +
                esc(missing && n === selected ? t('easydrag.ui.secret_missing', { name: n }) : n) + '</option>').join('');
            syncDelete();
        }
        // confirmReplace asks before the value of an existing secret is overwritten.
        async function confirmReplace(name) {
            const answer = await c.modal(env.root, {
                title: t('easydrag.ui.secret_replace_title'), closeLabel: t('easydrag.ui.close'),
                body: '<p>' + esc(t('easydrag.ui.secret_replace_text', { name })) + '</p>',
                actions: [{ id: 'keep', label: t('easydrag.ui.cancel') }, { id: 'replace', label: t('easydrag.ui.secret_replace'), danger: true }]
            }).done;
            return answer === 'replace';
        }
        select.addEventListener('change', () => { syncDelete(); onChange(select.value || undefined); });
        // The delete asks first: flows that use the secret fail until it is set again. The server
        // names the published ones (used_by), and a warning lists them. A 429 holds Delete for its
        // Retry-After, as Save does.
        if (deleteBtn) deleteBtn.addEventListener('click', () => {
            const name = select.value;
            if (!name) return;
            const dialog = c.modal(env.root, {
                title: t('easydrag.ui.secret_delete_title'), closeLabel: t('easydrag.ui.close'),
                body: '<p>' + esc(t('easydrag.ui.secret_delete_text', { name })) + '</p><p class="ed-error" role="alert" hidden></p>',
                actions: [{ id: 'keep', label: t('easydrag.ui.cancel') }, { id: 'delete', label: t('easydrag.ui.delete'), danger: true }],
                onAction: async (action, d) => {
                    if (action !== 'delete') return true;
                    let res;
                    try {
                        res = await env.api.deleteSecret(name);
                    } catch (err) {
                        const wait = err && err.retryAfter > 0 ? err.retryAfter : 0;
                        if (wait) holdAction(d, 'delete', wait);
                        const error = d.body.querySelector('.ed-error');
                        error.hidden = false;
                        error.textContent = c.errorText(t, err) + (wait ? ' ' + t('easydrag.ui.secret_retry', { seconds: wait }) : '');
                        return false;
                    }
                    if (env.secretCache) env.secretCache.names = null;
                    const cleared = select.value === name;
                    await load(cleared ? undefined : select.value);
                    if (cleared) onChange(undefined);
                    const users = res && Array.isArray(res.used_by) ? res.used_by.filter(n => typeof n === 'string') : [];
                    if (users.length && typeof env.notify === 'function') {
                        env.notify({ title: t('easydrag.ui.secret_deleted', { name }), message: t('easydrag.ui.secret_still_used', { flows: namesText(t, users) }), type: 'warning' });
                    }
                    return true;
                }
            });
            // The disabled Delete cannot keep the focus the dialog gives back: the list takes it. The
            // change rebuilt the form meanwhile (forms refresh), so the list is found again by its id.
            dialog.done.then(result => {
                if (result !== 'delete') return;
                const list = select.isConnected ? select : (id ? document.getElementById(id) : null);
                if (list) list.focus();
            });
        });
        const newBtn = el.querySelector('[data-ed-secret-new]');
        if (newBtn) newBtn.addEventListener('click', () => {
            const dialog = c.modal(env.root, {
                title: t('easydrag.ui.secret_new'), closeLabel: t('easydrag.ui.close'),
                body: '<label class="ed-label">' + esc(t('easydrag.ui.secret_name')) + '<input class="ed-input" data-ed-secret-name pattern="[a-z0-9_]{1,40}" autofocus></label>' +
                    '<label class="ed-label">' + esc(t('easydrag.ui.secret_value')) + '<input class="ed-input" type="password" data-ed-secret-value autocomplete="new-password"></label>' +
                    '<p class="ed-hint">' + esc(t('easydrag.ui.secret_hint')) + '</p><p class="ed-error" role="alert" hidden></p>',
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'save', label: t('easydrag.ui.save'), primary: true }],
                onAction: async (action, d) => {
                    if (action !== 'save') return true;
                    const nameInput = d.body.querySelector('[data-ed-secret-name]');
                    const name = nameInput.value.trim();
                    const secret = d.body.querySelector('[data-ed-secret-value]').value;
                    const error = d.body.querySelector('.ed-error');
                    const fail = text => { error.hidden = false; error.textContent = text; return false; };
                    // The server's own checks, before anything is sent.
                    if (!/^[a-z0-9_]{1,40}$/.test(name)) return fail(t('easydrag.ui.secret_invalid'));
                    if (!secret.trim()) return fail(t('easydrag.ui.secret_empty'));
                    if (utf8Bytes(secret) > SECRET_MAX_BYTES) return fail(t('easydrag.ui.secret_too_large'));
                    // A fresh list: another window may have created the name since the form was built.
                    if (env.secretCache) env.secretCache.names = null;
                    let names;
                    try { names = await secretNames(env); } catch (err) { return fail(t('easydrag.ui.secrets_unavailable')); }
                    if (names.includes(name)) {
                        fail(t('easydrag.ui.secret_name_taken', { name }));
                        if (!(await confirmReplace(name))) { nameInput.focus(); return false; }
                    }
                    try {
                        await env.api.saveSecret(name, secret);
                    } catch (err) {
                        const wait = err && err.retryAfter > 0 ? err.retryAfter : 0;
                        if (wait) holdAction(d, 'save', wait);
                        return fail(c.errorText(t, err) + (wait ? ' ' + t('easydrag.ui.secret_retry', { seconds: wait }) : ''));
                    }
                    if (env.secretCache) env.secretCache.names = null;
                    await load(name);
                    onChange(name);
                    return true;
                }
            });
            return dialog;
        });
        load(value);
        return el;
    }

    ED.fields = { templateField, chipsMarkup, sampleAt, segmented, toggle, selectBox, conditionGroup, cases, fieldList, decisionQuestions, decisionImages, keyValue, jsonEditor, tags, dateTime, fileField, secretRef, OPS, UNARY };
})();
