// EasyDrag template field: a text with {{…}} references shown as chips, edited with {{
// autocomplete and caret-aware drops, plus the sample lookup its suggestions use. It loads
// before easydrag-fields-conditions.js and easydrag-fields.js and defines ED.fieldsShared,
// the helpers all three share.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    // fieldCount numbers the element ids of every parameter editor (suggestion lists here, key-value
    // and JSON errors in easydrag-fields.js): one counter, so the ids stay as before the split.
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

    // composing reports a key event that belongs to an IME composition (Enter picks a word).
    function composing(event) { return event.isComposing || event.keyCode === 229; }

    // nextFieldId returns the next number for a parameter editor's element ids.
    function nextFieldId() { return ++fieldCount; }

    ED.fieldsShared = { nameAttrs, withChange, composing, nextFieldId };

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
        const applyChange = env.change;
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
                applyChange(current);
            }
        }

        // flush saves an open edit and leaves the caret, the suggestion list and is-editing
        // alone. False means an IME composition still owns the text.
        function flush() {
            if (!el.classList.contains('is-editing')) return true;
            if (input.edComposing) return false;
            commit();
            return true;
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
        input.addEventListener('compositionstart', () => { input.edComposing = true; });
        input.addEventListener('compositionend', () => { input.edComposing = false; });
        input.addEventListener('blur', () => {
            setTimeout(() => {
                if (el.contains(document.activeElement)) return;
                closeSuggest();
                if (input.edComposing) return;
                // commit uses the change captured for this field, not a step opened later.
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
                // A tab switch blurs the editor, but its unchanged buffer retains the caret.
                if (el.classList.contains('is-editing') || input.value === current) {
                    const pos = input.selectionStart;
                    input.value = input.value.slice(0, pos) + token + input.value.slice(input.selectionEnd);
                    input.setSelectionRange(pos + token.length, pos + token.length);
                    commit();
                    if (!el.classList.contains('is-editing')) syncView();
                    return;
                }
                current = current ? (multiline ? current + ' ' + token : current + token) : token;
                applyChange(current);
                syncView();
            },
            focus: edit,
            refresh: syncView,
            flush
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

    ED.fieldsTemplate = { templateField, chipsMarkup, sampleAt };
})();
