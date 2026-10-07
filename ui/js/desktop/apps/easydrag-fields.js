// EasyDrag parameter editors: the template field (chips, {{ autocomplete, drop target) and
// the complex editors (conditions, switch cases, field lists, key-value, JSON, tags, file,
// secrets, date and time). Every editor calls env.change(value) on edits.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const OPS = ['eq', 'ne', 'contains', 'not_contains', 'starts_with', 'ends_with', 'matches', 'gt', 'gte', 'lt', 'lte', 'before', 'after', 'empty', 'not_empty', 'is_true', 'is_false'];
    const UNARY = new Set(['empty', 'not_empty', 'is_true', 'is_false']);
    const COND_TYPES = ['auto', 'text', 'number', 'date', 'bool'];
    // SECRET_MAX_BYTES is the server's limit for a flow secret value (UTF-8 bytes).
    const SECRET_MAX_BYTES = 4096;
    let fieldCount = 0;

    function core() { return ED.core; }

    // nameAttrs names a control: aria-labelledby (a.labelledBy, an element id) or aria-label
    // (a.label, a text).
    function nameAttrs(esc, a) {
        if (a && a.labelledBy) return ' aria-labelledby="' + esc(a.labelledBy) + '"';
        if (a && a.label) return ' aria-label="' + esc(a.label) + '"';
        return '';
    }

    // withChange returns env with its own change handler. Everything else (roots, upstream)
    // is still read through env, so a sub-field sees data that arrives later.
    function withChange(env, change) { return Object.assign(Object.create(env), { change }); }

    // objects keeps the object items of a list; a malformed param becomes an empty list.
    function objects(list) { return Array.isArray(list) ? list.filter(x => x && typeof x === 'object' && !Array.isArray(x)) : []; }

    function blankRow() { return { left: '', op: 'eq', right: '', type: 'auto' }; }

    // composing reports a key event that belongs to an IME composition (Enter picks a word).
    function composing(event) { return event.isComposing || event.keyCode === 229; }

    // trimTrailingDots drops a "." left at the end of an expression ("{{alpha.}}" once a step
    // was picked from the suggestions), so the saved text parses. Only a dot right before the
    // closing braces and after a name or "]" goes; literal text and quoted strings stay.
    function trimTrailingDots(text) {
        let out = text;
        const exprs = ED.template.segments(text).filter(s => s.expr !== undefined);
        for (let i = exprs.length - 1; i >= 0; i--) {
            const s = exprs[i];
            const fixed = s.expr.replace(/([A-Za-z0-9_\]])\.(\s*)$/, '$1$2');
            if (fixed !== s.expr) out = out.slice(0, s.start + 2) + fixed + out.slice(s.end - 2);
        }
        return out;
    }

    // ── template field ──────────────────────────────────────────────────────────

    // chipsMarkup renders text with {{…}} expressions as chips labelled with the node label.
    function chipsMarkup(text, env) {
        const { esc } = env;
        return ED.template.segments(text).map(seg => {
            if (seg.expr === undefined) return esc(seg.literal);
            let label = seg.expr.trim();
            let cls = 'ed-ref';
            try {
                const p = ED.template.parseExpr(seg.expr);
                const src = env.upstream.find(u => u.key === p.root);
                if (src) label = src.label + (p.path.length ? ' › ' + p.path.join(' › ') : '');
                else if (['trigger', 'run', 'flow'].includes(p.root)) label = core().tr(env.t, 'easydrag.ui.root_' + p.root, p.root) + (p.path.length ? ' › ' + p.path.join(' › ') : '');
                else cls += ' is-unknown';
                if (p.filters.length) label += ' | ' + p.filters.map(f => f.name).join(' | ');
                if (src) cls += ' cat-' + esc(src.cat);
            } catch (err) { cls += ' is-broken'; }
            return '<span class="' + cls + '" title="' + esc('{{' + seg.expr + '}}') + '">' + esc(label) + '</span>';
        }).join('');
    }

    // templateField returns {el, get, set, insert, focus}. opts: {multiline, placeholder, id,
    // labelledBy, label}; labelledBy (an element id) or label (a text) names the field.
    function templateField(env, value, opts) {
        const c = core();
        const { t, esc } = env;
        const o = opts || {};
        const multiline = !!o.multiline;
        const listId = 'ed-suggest-' + (++fieldCount);
        const named = nameAttrs(esc, o);
        // A single line edits like a combo box; a multi-line field stays a text box that
        // points at its suggestion list.
        const el = c.el('<div class="ed-tpl' + (multiline ? ' ed-tpl--multi' : '') + '" data-ed-drop="1">' +
            '<div class="ed-tpl-view" tabindex="0" role="textbox"' + named + (multiline ? ' aria-multiline="true"' : '') + (env.readonly ? ' aria-readonly="true"' : '') + '></div>' +
            '<textarea class="ed-tpl-input" rows="' + (multiline ? 4 : 1) + '" spellcheck="' + (multiline ? 'true' : 'false') + '"' +
            (o.id ? ' id="' + esc(o.id) + '"' : '') + named + (multiline ? '' : ' role="combobox" aria-expanded="false"') +
            ' aria-autocomplete="list" placeholder="' + esc(o.placeholder || '') + '"></textarea></div>');
        const view = el.querySelector('.ed-tpl-view');
        const input = el.querySelector('.ed-tpl-input');
        let current = value == null ? '' : String(value);
        let suggest = null;

        function syncView() {
            view.innerHTML = current ? chipsMarkup(current, env) : '<span class="ed-tpl-placeholder">' + esc(o.placeholder || '') + '</span>';
            el.classList.toggle('has-refs', current.indexOf('{{') >= 0);
        }

        function grow() {
            input.style.height = 'auto';
            input.style.height = Math.min(multiline ? 320 : 120, input.scrollHeight + 2) + 'px';
        }

        function edit() {
            if (env.readonly) return;
            el.classList.add('is-editing');
            input.value = current;
            grow();
            input.focus();
        }

        function commit() {
            const next = trimTrailingDots(multiline ? input.value : input.value.replace(/\n/g, ' '));
            if (next !== current) {
                current = next;
                env.change(current);
            }
        }

        // leave ends editing from the keyboard and moves focus to the chips, so it never drops
        // to the page. save false undoes the edit.
        function leave(save) {
            closeSuggest();
            if (save) commit(); else input.value = current;
            el.classList.remove('is-editing');
            syncView();
            view.focus();
        }

        function closeSuggest() {
            if (!suggest) return;
            suggest.remove();
            suggest = null;
            if (!multiline) input.setAttribute('aria-expanded', 'false');
            input.removeAttribute('aria-controls');
            input.removeAttribute('aria-activedescendant');
        }

        function markActive() {
            suggest.querySelectorAll('li').forEach((li, i) => {
                li.classList.toggle('is-active', i === suggest.active);
                li.setAttribute('aria-selected', String(i === suggest.active));
            });
            input.setAttribute('aria-activedescendant', listId + '-' + suggest.active);
        }

        function suggestions(prefix) {
            const p = prefix.replace(/^\s+/, '');
            const pipe = p.lastIndexOf('|');
            if (pipe >= 0) {
                const q = p.slice(pipe + 1).trim();
                return ED.template.FILTER_LIST.filter(f => f.name.startsWith(q)).map(f => ({ text: p.slice(0, pipe + 1) + ' ' + f.name + (f.args ? f.hint : ''), label: f.name + f.hint, kind: 'filter' }));
            }
            const dot = p.lastIndexOf('.');
            if (dot < 0) {
                const roots = env.upstream.map(u => ({ key: u.key, label: u.label, cat: u.cat })).concat(['trigger', 'run', 'flow'].map(k => ({ key: k, label: c.tr(t, 'easydrag.ui.root_' + k, k), cat: 'root' })));
                return roots.filter(r => r.key.startsWith(p) || r.label.toLowerCase().includes(p.toLowerCase())).map(r => ({ text: r.key, label: r.label + ' · ' + r.key, kind: r.cat }));
            }
            const base = p.slice(0, dot);
            const q = p.slice(dot + 1);
            const rootKey = base.split(/[.[]/)[0];
            const src = env.upstream.find(u => u.key === rootKey);
            let names = [];
            // Output fields come as names or as {name, type} (the shape the input tree uses).
            if (src && base === rootKey) names = (src.fields || []).map(f => (f && typeof f === 'object' ? f.name : f)).filter(n => typeof n === 'string');
            const sample = sampleAt(env, base);
            if (sample && typeof sample === 'object' && !Array.isArray(sample)) names = Array.from(new Set(names.concat(Object.keys(sample))));
            if (rootKey === 'trigger' && base === 'trigger') names = ['data', 'fired_at', 'type', 'node'];
            if (rootKey === 'run' && base === 'run') names = ['id', 'started_at', 'mode', 'revision'];
            if (rootKey === 'flow' && base === 'flow') names = ['id', 'name'];
            // Names that are no plain identifiers ("content-type") go in quotes.
            return names.filter(n => n.startsWith(q)).map(n => ({ text: ED.template.pathJoin(base, n), label: n, kind: 'field' }));
        }

        function showSuggest() {
            closeSuggest();
            const before = input.value.slice(0, input.selectionStart);
            const m = /\{\{([^{}]*)$/.exec(before);
            if (!m) return;
            // Only what differs from the text typed so far is worth offering: a list that
            // repeats it would take Enter and Tab away from the field.
            const typed = m[1].replace(/^\s+/, '');
            const items = suggestions(m[1]).filter(it => it.text !== typed).slice(0, 12);
            if (!items.length) return;
            suggest = c.el('<ul class="ed-suggest" role="listbox" id="' + listId + '"></ul>');
            suggest.innerHTML = items.map((it, i) => '<li role="option" id="' + listId + '-' + i + '" aria-selected="' + (i === 0) + '" data-ed-sug="' + i + '" class="' + (i === 0 ? 'is-active' : '') + '">' +
                '<span class="ed-suggest-kind" data-kind="' + esc(it.kind) + '"></span>' + esc(it.label) + '</li>').join('');
            suggest.items = items;
            suggest.active = 0;
            el.appendChild(suggest);
            if (!multiline) input.setAttribute('aria-expanded', 'true');
            input.setAttribute('aria-controls', listId);
            input.setAttribute('aria-activedescendant', listId + '-0');
        }

        // applySuggest puts a suggestion at the caret. A step or an object value goes on to its
        // fields ("alpha." and the list of them); anything else closes the list.
        function applySuggest(idx) {
            const it = suggest && suggest.items[idx];
            if (!it) return;
            const pos = input.selectionStart;
            const before = input.value.slice(0, pos);
            const after = input.value.slice(pos);
            const start = before.lastIndexOf('{{') + 2;
            const closing = after.trimStart().startsWith('}}') ? '' : '}}';
            const deeper = it.kind === 'filter' ? [] : suggestions(it.text + '.');
            const text = deeper.length ? it.text + '.' : it.text;
            input.value = before.slice(0, start) + text + closing + after;
            const caret = start + text.length;
            input.setSelectionRange(caret, caret);
            closeSuggest();
            grow();
            if (deeper.length) showSuggest();
        }

        view.addEventListener('click', edit);
        view.addEventListener('keydown', (event) => { if (event.key === 'Enter' || event.key === 'F2') { event.preventDefault(); edit(); } });
        input.addEventListener('input', () => { grow(); showSuggest(); });
        input.addEventListener('keydown', (event) => {
            if (composing(event)) return;
            if (suggest) {
                if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                    event.preventDefault();
                    suggest.active = (suggest.active + (event.key === 'ArrowDown' ? 1 : -1) + suggest.items.length) % suggest.items.length;
                    markActive();
                    return;
                }
                if (event.key === 'Enter' || event.key === 'Tab') { event.preventDefault(); applySuggest(suggest.active); return; }
                if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closeSuggest(); return; }
            }
            // Enter saves a single line; Escape saves a multi-line text (like leaving it) and
            // undoes a single line.
            if (event.key === 'Enter' && !multiline) { event.preventDefault(); leave(true); }
            else if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); leave(multiline); }
        });
        el.addEventListener('pointerdown', (event) => {
            const li = event.target.closest('[data-ed-sug]');
            if (li) { event.preventDefault(); applySuggest(Number(li.dataset.edSug)); }
        });
        input.addEventListener('blur', () => {
            setTimeout(() => {
                if (el.contains(document.activeElement)) return;
                closeSuggest();
                commit();
                el.classList.remove('is-editing');
                syncView();
            }, 120);
        });
        syncView();

        const api = {
            el,
            get: () => current,
            set(v) { current = v == null ? '' : String(v); if (!el.classList.contains('is-editing')) syncView(); },
            // insert puts a reference at the caret (or appends it) and saves.
            insert(ref) {
                const token = '{{' + ref + '}}';
                if (el.classList.contains('is-editing')) {
                    const pos = input.selectionStart;
                    input.value = input.value.slice(0, pos) + token + input.value.slice(input.selectionEnd);
                    input.setSelectionRange(pos + token.length, pos + token.length);
                    commit();
                    return;
                }
                current = current ? (multiline ? current + ' ' + token : current + token) : token;
                env.change(current);
                syncView();
            },
            focus: edit,
            refresh: syncView
        };
        // forms and drops reach the field through its element (caret-aware insert).
        el.edTemplate = api;
        return api;
    }

    // sampleAt reads the sample data at path ("key.list[-1]") like the engine: through the
    // shared resolver, so only own fields count, a negative index counts from the end and
    // anything missing (or a path that does not parse) is null.
    function sampleAt(env, path) {
        try {
            const p = ED.template.parseExpr(path);
            const roots = env.roots || {};
            return ED.template.resolvePath(Object.prototype.hasOwnProperty.call(roots, p.root) ? roots[p.root] : undefined, p.path);
        } catch (err) { return null; }
    }

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
            const rm = event.target.closest('[data-ed-case-remove]');
            if (rm) { list.splice(Number(rm.dataset.edCaseRemove), 1); emit(); render(); }
        });
        render();
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

    function keyValue(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const source = value && typeof value === 'object' && !Array.isArray(value) ? value : {};
        const pairs = Object.entries(source).map(([k, v]) => ({ k, v: v == null ? '' : typeof v === 'object' ? JSON.stringify(v) : String(v) }));
        const el = c.el('<div class="ed-kv"><div class="ed-kv-rows"></div>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-kv-add>' + c.icon('plus') + '<span>' + esc(t('easydrag.ui.kv_add')) + '</span></button>') + '</div>');
        const rows = el.querySelector('.ed-kv-rows');
        const emit = () => {
            const out = {};
            pairs.forEach(p => { if (p.k.trim()) out[p.k.trim()] = p.v; });
            onChange(out);
        };
        function render() {
            rows.innerHTML = '';
            pairs.forEach((p, idx) => {
                const row = c.el('<div class="ed-kv-row"><input class="ed-input" placeholder="' + esc(t('easydrag.ui.kv_key')) + '" aria-label="' + esc(t('easydrag.ui.kv_key')) + '" value="' + esc(p.k) + '"' + (env.readonly ? ' disabled' : '') + '><div class="ed-kv-value"></div>' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-kv-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove_row', { n: idx + 1 })) + '">' + c.icon('x') + '</button>') + '</div>');
                row.querySelector('input').addEventListener('change', (event) => { p.k = event.target.value; emit(); });
                row.querySelector('.ed-kv-value').appendChild(templateField(withChange(env, v => { p.v = v; emit(); }), p.v, { placeholder: t('easydrag.ui.kv_value'), label: t('easydrag.ui.kv_value') }).el);
                rows.appendChild(row);
            });
        }
        el.addEventListener('click', (event) => {
            if (event.target.closest('[data-ed-kv-add]')) { pairs.push({ k: '', v: '' }); render(); return; }
            const rm = event.target.closest('[data-ed-kv-remove]');
            if (rm) { pairs.splice(Number(rm.dataset.edKvRemove), 1); emit(); render(); }
        });
        render();
        return el;
    }

    function jsonEditor(env, value, onChange, id) {
        const c = core();
        const { t, esc } = env;
        const text = value === undefined ? '' : JSON.stringify(value, null, 2);
        const el = c.el('<div class="ed-json"><textarea class="ed-input ed-code" rows="6" spellcheck="false"' + (id ? ' id="' + esc(id) + '"' : '') + (env.readonly ? ' disabled' : '') + '>' + esc(text) + '</textarea><div class="ed-json-error" hidden></div></div>');
        const area = el.querySelector('textarea');
        const error = el.querySelector('.ed-json-error');
        area.addEventListener('input', () => {
            const raw = area.value.trim();
            if (!raw) { error.hidden = true; onChange(undefined); return; }
            try { const parsed = JSON.parse(raw); error.hidden = true; onChange(parsed); } catch (err) { error.hidden = false; error.textContent = t('easydrag.ui.json_invalid'); }
        });
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

    // holdAction keeps a dialog button disabled for seconds (a 429 with Retry-After). The dialog
    // enables its buttons once the action returns, so the hold starts after that.
    function holdAction(dialog, id, seconds) {
        const button = dialog.el.querySelector('[data-ed-action="' + id + '"]');
        if (!button || !(seconds > 0)) return;
        setTimeout(() => { button.disabled = true; }, 0);
        setTimeout(() => { button.disabled = false; }, Math.min(seconds, 3600) * 1000);
    }

    // namesText lists at most three names, and "and N more" for the rest.
    function namesText(t, names) {
        const shown = names.slice(0, 3).join(', ');
        return names.length > 3 ? t('easydrag.ui.names_more', { names: shown, count: names.length - 3 }) : shown;
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
            try {
                names = (await secretNames(env)).slice();
                unavailable.hidden = true;
                select.removeAttribute('data-error');
            } catch (err) {
                // Like a failed option list: the reason on the select, a visible hint under it.
                unavailable.hidden = false;
                select.dataset.error = c.errorText(t, err);
            }
            if (selected && !names.includes(selected)) names.push(selected);
            select.innerHTML = '<option value="">' + esc(t('easydrag.ui.secret_none')) + '</option>' + names.map(n => '<option value="' + esc(n) + '"' + (n === selected ? ' selected' : '') + '>' + esc(n) + '</option>').join('');
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
            // The disabled Delete cannot keep the focus the dialog gives back: the list takes it.
            dialog.done.then(result => { if (result === 'delete' && !select.value) select.focus(); });
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

    ED.fields = { templateField, chipsMarkup, sampleAt, segmented, toggle, selectBox, conditionGroup, cases, fieldList, keyValue, jsonEditor, tags, dateTime, fileField, secretRef, OPS, UNARY };
})();
