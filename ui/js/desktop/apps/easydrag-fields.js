// EasyDrag parameter editors: the template field (chips, {{ autocomplete, drop target) and
// the complex editors (conditions, switch cases, field lists, key-value, JSON, tags, file,
// secrets, date and time). Every editor calls env.change(value) on edits.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const OPS = ['eq', 'ne', 'contains', 'not_contains', 'starts_with', 'ends_with', 'matches', 'gt', 'gte', 'lt', 'lte', 'before', 'after', 'empty', 'not_empty', 'is_true', 'is_false'];
    const UNARY = new Set(['empty', 'not_empty', 'is_true', 'is_false']);
    const COND_TYPES = ['auto', 'text', 'number', 'date', 'bool'];

    function core() { return ED.core; }

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
                if (src) cls += ' cat-' + src.cat;
            } catch (err) { cls += ' is-broken'; }
            return '<span class="' + cls + '" title="' + esc('{{' + seg.expr + '}}') + '">' + esc(label) + '</span>';
        }).join('');
    }

    // templateField returns {el, get, set, insert, focus}. opts: {multiline, placeholder, id}.
    function templateField(env, value, opts) {
        const c = core();
        const { t, esc } = env;
        const multiline = !!(opts && opts.multiline);
        const el = c.el('<div class="ed-tpl' + (multiline ? ' ed-tpl--multi' : '') + '" data-ed-drop="1">' +
            '<div class="ed-tpl-view" tabindex="0" role="textbox" aria-readonly="true"></div>' +
            '<textarea class="ed-tpl-input" rows="' + (multiline ? 4 : 1) + '" spellcheck="' + (multiline ? 'true' : 'false') + '"' +
            (opts && opts.id ? ' id="' + esc(opts.id) + '"' : '') + ' placeholder="' + esc((opts && opts.placeholder) || '') + '"></textarea></div>');
        const view = el.querySelector('.ed-tpl-view');
        const input = el.querySelector('.ed-tpl-input');
        let current = value == null ? '' : String(value);
        let suggest = null;

        function syncView() {
            view.innerHTML = current ? chipsMarkup(current, env) : '<span class="ed-tpl-placeholder">' + esc((opts && opts.placeholder) || '') + '</span>';
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
            if (input.value !== current) {
                current = multiline ? input.value : input.value.replace(/\n/g, ' ');
                env.change(current);
            }
        }

        function closeSuggest() { if (suggest) { suggest.remove(); suggest = null; } }

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
            if (src && base === rootKey) names = src.fields.slice();
            const sample = sampleAt(env, base);
            if (sample && typeof sample === 'object' && !Array.isArray(sample)) names = Array.from(new Set(names.concat(Object.keys(sample))));
            if (rootKey === 'trigger' && base === 'trigger') names = ['data', 'fired_at', 'type', 'node'];
            if (rootKey === 'run' && base === 'run') names = ['id', 'started_at', 'mode', 'revision'];
            if (rootKey === 'flow' && base === 'flow') names = ['id', 'name'];
            return names.filter(n => n.startsWith(q)).map(n => ({ text: base + '.' + n, label: n, kind: 'field' }));
        }

        function showSuggest() {
            closeSuggest();
            const before = input.value.slice(0, input.selectionStart);
            const m = /\{\{([^{}]*)$/.exec(before);
            if (!m) return;
            const items = suggestions(m[1]).slice(0, 12);
            if (!items.length) return;
            suggest = c.el('<ul class="ed-suggest" role="listbox"></ul>');
            suggest.innerHTML = items.map((it, i) => '<li role="option" data-ed-sug="' + i + '" class="' + (i === 0 ? 'is-active' : '') + '"><span class="ed-suggest-kind" data-kind="' + esc(it.kind) + '"></span>' + esc(it.label) + '</li>').join('');
            suggest.items = items;
            suggest.active = 0;
            el.appendChild(suggest);
        }

        function applySuggest(idx) {
            const it = suggest && suggest.items[idx];
            if (!it) return;
            const pos = input.selectionStart;
            const before = input.value.slice(0, pos);
            const after = input.value.slice(pos);
            const start = before.lastIndexOf('{{') + 2;
            const closing = after.trimStart().startsWith('}}') ? '' : '}}';
            input.value = before.slice(0, start) + it.text + closing + after;
            const caret = start + it.text.length;
            input.setSelectionRange(caret, caret);
            closeSuggest();
            grow();
            if (it.kind !== 'filter') showSuggest();
        }

        view.addEventListener('click', edit);
        view.addEventListener('keydown', (event) => { if (event.key === 'Enter' || event.key === 'F2') { event.preventDefault(); edit(); } });
        input.addEventListener('input', () => { grow(); showSuggest(); });
        input.addEventListener('keydown', (event) => {
            if (suggest) {
                if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
                    event.preventDefault();
                    suggest.active = (suggest.active + (event.key === 'ArrowDown' ? 1 : -1) + suggest.items.length) % suggest.items.length;
                    suggest.querySelectorAll('li').forEach((li, i) => li.classList.toggle('is-active', i === suggest.active));
                    return;
                }
                if (event.key === 'Enter' || event.key === 'Tab') { event.preventDefault(); applySuggest(suggest.active); return; }
                if (event.key === 'Escape') { event.stopPropagation(); closeSuggest(); return; }
            }
            if (event.key === 'Enter' && !multiline) { event.preventDefault(); input.blur(); }
            if (event.key === 'Escape') { event.stopPropagation(); input.value = current; input.blur(); }
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

    function sampleAt(env, path) {
        try {
            const p = ED.template.parseExpr(path);
            let v = env.roots[p.root];
            for (const seg of p.path) { if (v == null) return null; v = v[seg]; }
            return v;
        } catch (err) { return null; }
    }

    // ── simple controls ─────────────────────────────────────────────────────────

    function segmented(env, options, value, onChange) {
        const { esc } = env;
        const el = core().el('<div class="ed-seg" role="radiogroup">' + options.map(o =>
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

    function selectBox(env, options, value, onChange, id) {
        const { esc } = env;
        const el = core().el('<select class="ed-input"' + (id ? ' id="' + esc(id) + '"' : '') + (env.readonly ? ' disabled' : '') + '>' +
            options.map(o => '<option value="' + esc(o.value) + '"' + (String(o.value) === String(value == null ? '' : value) ? ' selected' : '') + '>' + esc(o.label) + '</option>').join('') + '</select>');
        el.addEventListener('change', () => onChange(el.value));
        return el;
    }

    // ── condition group ─────────────────────────────────────────────────────────

    function conditionGroup(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const group = Object.assign({ match: 'all', rows: [] }, value && typeof value === 'object' ? JSON.parse(JSON.stringify(value)) : {});
        if (!group.rows.length) group.rows.push({ left: '', op: 'eq', right: '', type: 'auto' });
        const el = c.el('<div class="ed-cond"><div class="ed-cond-head"></div><div class="ed-cond-rows"></div>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-cond-add>' + c.icon('plus') + '<span>' + esc(t('easydrag.ui.cond_add')) + '</span></button>') + '</div>');
        const head = el.querySelector('.ed-cond-head');
        const rows = el.querySelector('.ed-cond-rows');
        const emit = () => onChange(JSON.parse(JSON.stringify(group)));

        head.appendChild(segmented(env, [{ value: 'all', label: t('easydrag.ui.cond_all') }, { value: 'any', label: t('easydrag.ui.cond_any') }], group.match, v => { group.match = v; emit(); }));

        function renderRows() {
            rows.innerHTML = '';
            group.rows.forEach((row, idx) => {
                const r = c.el('<div class="ed-cond-row"><div class="ed-cond-left"></div><div class="ed-cond-op"></div><div class="ed-cond-right"></div><div class="ed-cond-type"></div>' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-cond-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove')) + '">' + c.icon('x') + '</button>') + '</div>');
                const left = templateField(Object.assign({}, env, { change: v => { row.left = v; emit(); } }), row.left, { placeholder: t('easydrag.ui.cond_left') });
                r.querySelector('.ed-cond-left').appendChild(left.el);
                r.querySelector('.ed-cond-op').appendChild(selectBox(env, OPS.map(op => ({ value: op, label: c.tr(t, 'easydrag.ui.op_' + op, op) })), row.op, v => { row.op = v; emit(); renderRows(); }));
                if (!UNARY.has(row.op)) {
                    const right = templateField(Object.assign({}, env, { change: v => { row.right = v; emit(); } }), row.right, { placeholder: t('easydrag.ui.cond_right') });
                    r.querySelector('.ed-cond-right').appendChild(right.el);
                }
                r.querySelector('.ed-cond-type').appendChild(selectBox(env, COND_TYPES.map(ty => ({ value: ty, label: c.tr(t, 'easydrag.ui.cond_type_' + ty, ty) })), row.type || 'auto', v => { row.type = v; emit(); }));
                rows.appendChild(r);
            });
        }

        el.addEventListener('click', (event) => {
            if (event.target.closest('[data-ed-cond-add]')) { group.rows.push({ left: '', op: 'eq', right: '', type: 'auto' }); emit(); renderRows(); return; }
            const rm = event.target.closest('[data-ed-cond-remove]');
            if (rm) { group.rows.splice(Number(rm.dataset.edCondRemove), 1); if (!group.rows.length) group.rows.push({ left: '', op: 'eq', right: '', type: 'auto' }); emit(); renderRows(); }
        });
        renderRows();
        return el;
    }

    function cases(env, value, onChange) {
        const c = core();
        const { t, esc } = env;
        const list = Array.isArray(value) ? JSON.parse(JSON.stringify(value)) : [];
        const el = c.el('<div class="ed-cases"><div class="ed-cases-list"></div>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-case-add>' + c.icon('plus') + '<span>' + esc(t('easydrag.ui.case_add')) + '</span></button>') + '</div>');
        const host = el.querySelector('.ed-cases-list');
        const emit = () => onChange(JSON.parse(JSON.stringify(list)));
        function render() {
            host.innerHTML = '';
            list.forEach((cs, idx) => {
                const card = c.el('<div class="ed-case"><div class="ed-case-head"><span class="ed-case-port">' + esc(t('easydrag.ui.port_case', { n: idx + 1 })) + '</span>' +
                    '<input class="ed-input ed-case-label" placeholder="' + esc(t('easydrag.ui.case_label')) + '" value="' + esc(cs.label || '') + '"' + (env.readonly ? ' disabled' : '') + '>' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-case-up="' + idx + '" aria-label="' + esc(t('easydrag.ui.move_up')) + '"' + (idx === 0 ? ' disabled' : '') + '>' + c.icon('chevron-down', 'ed-rot180') + '</button>' +
                        '<button type="button" class="ed-icon-btn" data-ed-case-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove')) + '">' + c.icon('trash') + '</button>') + '</div></div>');
                card.querySelector('.ed-case-label').addEventListener('change', (event) => { cs.label = event.target.value; emit(); });
                card.appendChild(conditionGroup(env, cs.condition, v => { cs.condition = v; emit(); }));
                host.appendChild(card);
            });
        }
        el.addEventListener('click', (event) => {
            if (event.target.closest('[data-ed-case-add]')) { list.push({ label: '', condition: { match: 'all', rows: [{ left: '', op: 'eq', right: '', type: 'auto' }] } }); emit(); render(); return; }
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
        const list = Array.isArray(value) ? JSON.parse(JSON.stringify(value)) : [];
        const subs = spec.fields && spec.fields.length ? spec.fields : [{ name: 'name', kind: 'text', label: t('easydrag.ui.field_name') }, { name: 'value', kind: 'text', label: t('easydrag.ui.field_value'), templatable: true }];
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
                    if (s.options && s.options.length) cell.appendChild(selectBox(env, s.options, item[s.name] == null ? s.default : item[s.name], set));
                    else if (s.templatable) cell.appendChild(templateField(Object.assign({}, env, { change: set }), item[s.name], { placeholder: s.label }).el);
                    else {
                        const input = c.el('<input class="ed-input" value="' + esc(item[s.name] == null ? '' : item[s.name]) + '" placeholder="' + esc(s.label) + '" aria-label="' + esc(s.label) + '"' + (env.readonly ? ' disabled' : '') + '>');
                        input.addEventListener('change', () => set(input.value));
                        cell.appendChild(input);
                    }
                    row.appendChild(cell);
                });
                if (!env.readonly) row.appendChild(c.el('<button type="button" class="ed-icon-btn" data-ed-fl-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove')) + '">' + c.icon('x') + '</button>'));
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
        const pairs = Object.entries(value && typeof value === 'object' ? value : {}).map(([k, v]) => ({ k, v: v == null ? '' : String(v) }));
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
                const row = c.el('<div class="ed-kv-row"><input class="ed-input" placeholder="' + esc(t('easydrag.ui.kv_key')) + '" value="' + esc(p.k) + '"' + (env.readonly ? ' disabled' : '') + '><div class="ed-kv-value"></div>' +
                    (env.readonly ? '' : '<button type="button" class="ed-icon-btn" data-ed-kv-remove="' + idx + '" aria-label="' + esc(t('easydrag.ui.remove')) + '">' + c.icon('x') + '</button>') + '</div>');
                row.querySelector('input').addEventListener('change', (event) => { p.k = event.target.value; emit(); });
                row.querySelector('.ed-kv-value').appendChild(templateField(Object.assign({}, env, { change: v => { p.v = v; emit(); } }), p.v, { placeholder: t('easydrag.ui.kv_value') }).el);
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
        const list = Array.isArray(value) ? value.map(String) : [];
        const el = c.el('<div class="ed-tags"><div class="ed-tags-list"></div><input class="ed-input ed-tags-input" placeholder="' + esc(t('easydrag.ui.tags_add')) + '" enterkeyhint="done"' + (env.readonly ? ' disabled' : '') + '></div>');
        const host = el.querySelector('.ed-tags-list');
        const input = el.querySelector('input');
        function render() {
            host.innerHTML = list.map((v, i) => '<span class="ed-tag">' + esc(v) + (env.readonly ? '' : '<button type="button" data-ed-tag-remove="' + i + '" aria-label="' + esc(t('easydrag.ui.remove')) + '">' + c.icon('x') + '</button>') + '</span>').join('');
        }
        input.addEventListener('keydown', (event) => {
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
    function fileField(env, value, onChange, id) {
        const text = typeof value === 'object' && value ? (value.path || '') : value;
        const field = templateField(Object.assign({}, env, { change: onChange }), text, { id, placeholder: env.t('easydrag.ui.file_placeholder') });
        field.el.classList.add('ed-file');
        return field.el;
    }

    // secretRef picks a flow secret (vault entry easydrag_<name>) or creates a new one.
    function secretRef(env, value, onChange, id) {
        const c = core();
        const { t, esc } = env;
        const el = c.el('<div class="ed-secret"><select class="ed-input"' + (id ? ' id="' + esc(id) + '"' : '') + (env.readonly ? ' disabled' : '') + '></select>' +
            (env.readonly ? '' : '<button type="button" class="ed-btn ed-btn--ghost ed-btn--small" data-ed-secret-new>' + c.icon('key') + '<span>' + esc(t('easydrag.ui.secret_new')) + '</span></button>') + '</div>');
        const select = el.querySelector('select');
        async function load(selected) {
            let names = [];
            try { names = (await env.api.secrets()).secrets || []; } catch (err) { names = []; }
            if (selected && !names.includes(selected)) names.push(selected);
            select.innerHTML = '<option value="">' + esc(t('easydrag.ui.secret_none')) + '</option>' + names.map(n => '<option value="' + esc(n) + '"' + (n === selected ? ' selected' : '') + '>' + esc(n) + '</option>').join('');
        }
        select.addEventListener('change', () => onChange(select.value || undefined));
        const newBtn = el.querySelector('[data-ed-secret-new]');
        if (newBtn) newBtn.addEventListener('click', () => {
            const dialog = c.modal(env.root, {
                title: t('easydrag.ui.secret_new'), closeLabel: t('easydrag.ui.close'),
                body: '<label class="ed-label">' + esc(t('easydrag.ui.secret_name')) + '<input class="ed-input" data-ed-secret-name pattern="[a-z0-9_]{1,40}" autofocus></label>' +
                    '<label class="ed-label">' + esc(t('easydrag.ui.secret_value')) + '<input class="ed-input" type="password" data-ed-secret-value autocomplete="new-password"></label>' +
                    '<p class="ed-hint">' + esc(t('easydrag.ui.secret_hint')) + '</p><p class="ed-error" hidden></p>',
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'save', label: t('easydrag.ui.save'), primary: true }],
                onAction: async (action, d) => {
                    if (action !== 'save') return true;
                    const name = d.body.querySelector('[data-ed-secret-name]').value.trim();
                    const secret = d.body.querySelector('[data-ed-secret-value]').value;
                    const error = d.body.querySelector('.ed-error');
                    if (!/^[a-z0-9_]{1,40}$/.test(name) || !secret) { error.hidden = false; error.textContent = t('easydrag.ui.secret_invalid'); return false; }
                    try { await env.api.saveSecret(name, secret); } catch (err) { error.hidden = false; error.textContent = c.errorText(t, err); return false; }
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

    ED.fields = { templateField, chipsMarkup, segmented, toggle, selectBox, conditionGroup, cases, fieldList, keyValue, jsonEditor, tags, dateTime, fileField, secretRef, OPS, UNARY };
})();
