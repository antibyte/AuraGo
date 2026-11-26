// HomeDash editor form controls (M4 part 2; design-visual-system 6, contract C15/C20). Builders shared by the palette, inspector, look editor and
// dialogs: native elements painted from --vd-mat-* by desktop-app-homedash.css, labelled by <label for> or aria-label, 32 px high (44 px coarse).
// Each returns {el, set(value), focus()} (+ wrap when the control sits in a decorated box); set() never fires a callback, so the inspector patches
// values in place. segmented()/tabs() are roving groups (arrows move and select); roving()/gridMove() are their pure key maths. iconGrid() is a
// searchable group of aria-pressed icon buttons. formSheet() opens a form in an E.sheet with its own .hd-dlg-btn buttons. textIssue() is the
// client copy of the server's text rules. No DOM at load.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.icons || !E.sheet) throw new Error('hd-form.js: load hd-icons.js and hd-sheet.js first');
    const { el, setAttr, uid } = E.core, STEP = Object.freeze({ ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }), HORIZONTAL = Object.freeze(['ArrowLeft', 'ArrowRight', 'Home', 'End']);
    const runes = (v, max) => Array.from(String(v == null ? '' : v)).slice(0, max).join('');
    const hit = (ev, sel) => (ev && ev.target && typeof ev.target.closest === 'function' ? ev.target.closest(sel) : null);
    const FOLD = new RegExp('[' + String.fromCodePoint(0x300) + '-' + String.fromCodePoint(0x36f) + ']', 'g');
    // Case- and accent-insensitive text for search (an umlaut matches its base letter).
    const fold = s => String(s == null ? '' : s).normalize('NFD').replace(FOLD, '').toLowerCase();
    function roving(key, index, count) { if (!(count > 0)) return null; if (key === 'Home') return 0; if (key === 'End') return count - 1; const d = STEP[key]; return d ? (index + d + count) % count : null; }
    // Grouped grid: `sizes` are item counts of consecutive groups laid out in `cols` columns. Left/Right walk the flat order; Up/Down keep the
    // column and cross into the neighbouring group; Home/End jump to the ends; no wrap.
    function gridMove(key, index, sizes, cols) {
        const c = Math.max(1, cols | 0), list = (sizes || []).map(n => Math.max(0, n | 0)), total = list.reduce((a, b) => a + b, 0); if (!total) return null;
        const i = Math.min(Math.max(index | 0, 0), total - 1);
        if (key === 'Home') return 0; if (key === 'End') return total - 1;
        if (key === 'ArrowLeft') return Math.max(0, i - 1); if (key === 'ArrowRight') return Math.min(total - 1, i + 1);
        if (key !== 'ArrowUp' && key !== 'ArrowDown') return null;
        let g = 0, start = 0; while (i >= start + list[g]) { start += list[g]; g += 1; }
        const col = (i - start) % c, row = Math.floor((i - start) / c), rows = n => Math.ceil(n / c), first = k => list.slice(0, k).reduce((a, b) => a + b, 0);
        const at = (k, r) => first(k) + Math.min(list[k] - 1, r * c + col);
        if (key === 'ArrowDown') { if (row + 1 < rows(list[g])) return at(g, row + 1); for (let k = g + 1; k < list.length; k++) if (list[k]) return at(k, 0); return i; }
        if (row > 0) return at(g, row - 1); for (let k = g - 1; k >= 0; k--) if (list[k]) return at(k, rows(list[k]) - 1); return i;
    }
    // Server text rules (internal/homedash/sanitize.go ContainsControl + ContainsURL). Returns '' when the server accepts `s`, else the issue code
    // ('control_chars' | 'url_not_allowed') that homedash.error.<code> explains. Titles, string options, page and dashboard names use it inline,
    // so the save lane never sticks in 'invalid' on text the user typed.
    const CTRL = /[\x00-\x1f\x7f-\x9f]/;
    const BIDI = new RegExp('[' + [0x061c, 0x200e, 0x200f].map(c => String.fromCodePoint(c)).join('') + String.fromCodePoint(0x202a) + '-' + String.fromCodePoint(0x202e) + String.fromCodePoint(0x2066) + '-' + String.fromCodePoint(0x2069) + ']');
    const URLISH = Object.freeze(['://', 'javascript:', 'data:', 'vbscript:']);
    function loneSurrogate(v) { for (let i = 0; i < v.length; i++) { const c = v.charCodeAt(i);
        if (c >= 0xd800 && c <= 0xdbff) { const d = v.charCodeAt(i + 1); if (d >= 0xdc00 && d <= 0xdfff) { i += 1; continue; } return true; } if (c >= 0xdc00 && c <= 0xdfff) return true; } return false; }
    function textIssue(s) {
        const v = String(s == null ? '' : s); if (CTRL.test(v) || BIDI.test(v) || loneSurrogate(v)) return 'control_chars';
        const low = v.toLowerCase(); return URLISH.some(m => low.includes(m)) || low.trim().startsWith('//') ? 'url_not_allowed' : '';
    }

    // ---- DOM builders (essential structure; classes and events are the contract of desktop-app-homedash.css) ----
    const ico = name => E.icons.icon(name), labelable = n => !!n && /^(input|select|button|textarea)$/i.test(n.localName || '');
    function field(o) {
        const x = o || {}, id = uid('hd-f'), c = x.control || {}, ctl = c.el || null, box = x.wrap || c.wrap || ctl;
        const label = el('label', { class: 'hd-f-label', id: id + '-l' }, [el('span', { text: x.label || '' }), x.required ? el('span', { class: 'hd-f-req', text: x.required === true ? '*' : x.required }) : null]);
        const help = x.help ? el('p', { class: 'hd-f-help', id: id + '-h', text: x.help }) : null, err = el('p', { class: 'hd-f-error', id: id + '-e', hidden: true });
        const root = el('div', { class: 'hd-f', 'data-hd-inline': x.inline ? 'true' : null }, [label, box, help, err]);
        if (ctl) { if (!ctl.id) ctl.id = id + '-c'; if (labelable(ctl)) label.setAttribute('for', ctl.id); else setAttr(ctl, 'aria-labelledby', id + '-l'); }
        const describe = shown => { if (ctl) setAttr(ctl, 'aria-describedby', [help ? id + '-h' : '', shown ? id + '-e' : ''].filter(Boolean).join(' ') || null); };
        describe(false);
        function error(text) { const on = !!text; err.textContent = on ? String(text) : ''; setAttr(err, 'hidden', on ? null : true); if (ctl) setAttr(ctl, 'aria-invalid', on ? true : null); describe(on); }
        return { el: root, control: c, error };
    }
    function text(o) {
        const x = o || {}, max = x.max || 200, input = el('input', { class: 'hd-f-input', type: x.search ? 'search' : 'text', spellcheck: 'false', autocomplete: 'off', 'data-hd-field': x.key, 'aria-label': x.label, placeholder: x.placeholder });
        input.value = x.value == null ? '' : String(x.value);
        const cut = () => { const v = runes(input.value, max); if (v !== input.value) input.value = v; return v; };
        let composing = false;
        input.addEventListener('compositionstart', () => { composing = true; });
        input.addEventListener('compositionend', () => { composing = false; const v = cut(); if (x.onInput) x.onInput(v); });
        input.addEventListener('input', ev => { if (composing || (ev && ev.isComposing)) return; const v = cut(); if (x.onInput) x.onInput(v); });
        input.addEventListener('change', () => { if (x.onCommit) x.onCommit(cut()); });
        input.addEventListener('blur', () => { if (x.onBlur) x.onBlur(input.value); });
        input.addEventListener('keydown', ev => { if (ev.key === 'Enter' && !ev.isComposing) { ev.preventDefault(); if (x.onEnter) x.onEnter(cut()); } });
        return { el: input, set(v) { input.value = v == null ? '' : String(v); }, get: () => input.value, focus() { input.focus(); if (x.select && typeof input.select === 'function') input.select(); } };
    }
    function number(o) {
        const x = o || {}, step = x.step === undefined || x.step === null ? 'any' : x.step;
        const input = el('input', { class: 'hd-f-input', type: 'number', inputmode: step === 1 ? 'numeric' : 'decimal', step: String(step), min: x.min, max: x.max, placeholder: x.placeholder, 'aria-label': x.label, 'data-hd-field': x.key });
        input.value = x.value == null ? '' : String(x.value); let last = input.value;
        const commit = () => { if (input.value === last) return; last = input.value; if (x.onCommit) x.onCommit(input.value); };
        input.addEventListener('change', commit); input.addEventListener('keydown', ev => { if (ev.key === 'Enter') { ev.preventDefault(); commit(); } });
        const wrap = el('div', { class: 'hd-f-num' }, [input, x.unit ? el('span', { class: 'hd-f-unit', 'aria-hidden': 'true', text: x.unit }) : null]);
        return { el: input, wrap, set(v) { input.value = v == null ? '' : String(v); last = input.value; }, focus() { input.focus(); } };
    }
    function select(o) {
        const x = o || {}, sel = el('select', { class: 'hd-f-select', 'aria-label': x.label, 'data-hd-field': x.key });
        const fill = (list, value) => { sel.replaceChildren(); for (const it of list || []) sel.appendChild(el('option', { value: it.id, disabled: it.disabled ? true : null, text: it.label })); sel.value = value == null ? '' : String(value); };
        fill(x.options, x.value); sel.addEventListener('change', () => { if (x.onChange) x.onChange(sel.value); });
        const wrap = el('div', { class: 'hd-f-selectwrap' }, [sel, ico('chevron-down')]);
        return { el: sel, wrap, set(v) { sel.value = v == null ? '' : String(v); }, options: fill, focus() { sel.focus(); } };
    }
    function toggle(o) {
        const x = o || {}, b = el('button', { type: 'button', class: 'hd-f-switch', role: 'switch', 'aria-checked': x.value === true ? 'true' : 'false', 'aria-label': x.label, 'data-hd-field': x.key }, [el('span', { class: 'hd-f-switch-track', 'aria-hidden': 'true' }, el('span', { class: 'hd-f-switch-thumb' }))]);
        b.addEventListener('click', () => { const on = b.getAttribute('aria-checked') !== 'true'; b.setAttribute('aria-checked', on ? 'true' : 'false'); if (x.onChange) x.onChange(on); });
        return { el: b, set(v) { b.setAttribute('aria-checked', v === true ? 'true' : 'false'); }, focus() { b.focus(); } };
    }
    function segmented(o) {
        const x = o || {}, root = el('div', { class: 'hd-f-seg', role: 'radiogroup', 'aria-label': x.label, 'aria-labelledby': x.labelledBy, 'data-hd-field': x.key });
        let cur = x.value == null ? '' : String(x.value);
        const btns = (x.options || []).map(opt => { const iconOnly = !!opt.iconOnly, b = el('button', { type: 'button', class: 'hd-f-seg-opt', role: 'radio', 'data-value': String(opt.id), 'aria-checked': 'false', tabindex: '-1',
            'aria-label': iconOnly ? opt.label : null, title: iconOnly ? opt.label : null }, [opt.icon ? ico(opt.icon) : null, iconOnly ? null : el('span', { text: opt.label })]); root.appendChild(b); return b; });
        const paint = () => { const on = btns.findIndex(b => b.getAttribute('data-value') === cur); btns.forEach((b, i) => { b.setAttribute('aria-checked', i === on ? 'true' : 'false'); b.setAttribute('tabindex', i === (on < 0 ? 0 : on) ? '0' : '-1'); }); };
        const choose = i => { const b = btns[i]; if (!b) return; cur = b.getAttribute('data-value'); paint(); b.focus(); if (x.onChange) x.onChange(cur); };
        root.addEventListener('click', ev => { const b = hit(ev, '.hd-f-seg-opt'); if (b && btns.includes(b)) choose(btns.indexOf(b)); });
        root.addEventListener('keydown', ev => { const at = btns.findIndex(b => b === ev.target || (ev.target && b.contains && b.contains(ev.target))), n = roving(ev.key, Math.max(0, at), btns.length); if (n === null || at < 0) return; ev.preventDefault(); choose(n); });
        paint(); return { el: root, set(v) { cur = v == null ? '' : String(v); paint(); }, focus() { const b = btns.find(y => y.getAttribute('tabindex') === '0') || btns[0]; if (b) b.focus(); } };
    }
    const parseHex = v => { const m = /^#?([0-9a-f]{6})$/i.exec(String(v == null ? '' : v).trim()); return m ? '#' + m[1].toLowerCase() : null; };
    function colour(o) {
        const x = o || {}, sw = el('input', { type: 'color', class: 'hd-f-swatch', 'aria-label': x.label, 'data-hd-field': x.key ? x.key + ':swatch' : null });
        const hex = el('input', { type: 'text', class: 'hd-f-input hd-f-hex', maxlength: '7', spellcheck: 'false', autocomplete: 'off', 'aria-label': x.label, placeholder: x.placeholder, 'data-hd-field': x.key });
        const root = el('div', { class: 'hd-f-colour' }, [sw, hex]);
        const paint = v => { const c = parseHex(v); hex.value = c || ''; sw.value = c || '#000000'; setAttr(root, 'data-hd-empty', c ? null : 'true'); return c; };
        sw.addEventListener('input', () => { const c = paint(sw.value); if (c && x.onInput) x.onInput(c); }); sw.addEventListener('change', () => { const c = paint(sw.value); if (c && x.onCommit) x.onCommit(c); });
        const fromText = () => { const v = hex.value.trim(); if (!v) { if (x.allowEmpty) { paint(''); if (x.onCommit) x.onCommit(''); } else if (x.onInvalid) x.onInvalid(v); return; }
            const c = parseHex(v); if (c) { paint(c); if (x.onCommit) x.onCommit(c); } else if (x.onInvalid) x.onInvalid(v); };
        hex.addEventListener('change', fromText); hex.addEventListener('keydown', ev => { if (ev.key === 'Enter') { ev.preventDefault(); fromText(); } });
        if (x.allowEmpty) root.appendChild(button({ label: x.clearLabel || '', icon: 'close', iconOnly: true, tone: 'quiet', onClick: () => { paint(''); if (x.onCommit) x.onCommit(''); } }).el);
        paint(x.value); return { el: hex, wrap: root, set: paint, focus() { hex.focus(); } };
    }
    function range(o) {
        const x = o || {}, input = el('input', { type: 'range', class: 'hd-f-range', min: x.min, max: x.max, step: x.step, 'aria-label': x.label, 'data-hd-field': x.key }), out = el('output', { class: 'hd-f-out', 'aria-hidden': 'true' });
        const sync = () => { const v = Number(input.value), txt = x.valueText ? x.valueText(v) : String(v); out.textContent = txt; setAttr(input, 'aria-valuetext', txt); };
        input.value = String(x.value == null ? x.min : x.value); sync();
        input.addEventListener('input', () => { sync(); if (x.onInput) x.onInput(Number(input.value)); }); input.addEventListener('change', () => { sync(); if (x.onCommit) x.onCommit(Number(input.value)); });
        return { el: input, wrap: el('div', { class: 'hd-f-rangewrap' }, [x.track || null, input, out]), set(v) { input.value = String(v); sync(); }, focus() { input.focus(); } };
    }
    function tabs(o) {
        const x = o || {}, base = uid('hd-tab'), root = el('div', { class: 'hd-f-tabs', role: 'tablist', 'aria-label': x.label });
        let cur = x.value; const idOf = id => base + '-' + id;
        const btns = (x.tabs || []).map(t => { const b = el('button', { type: 'button', class: 'hd-f-tab', role: 'tab', id: idOf(t.id), 'data-tab': t.id, 'aria-selected': 'false', 'aria-controls': x.controls, tabindex: '-1', text: t.label }); root.appendChild(b); return b; });
        const paint = () => btns.forEach(b => { const on = b.getAttribute('data-tab') === cur; b.setAttribute('aria-selected', on ? 'true' : 'false'); b.setAttribute('tabindex', on ? '0' : '-1'); });
        const choose = (i, focus) => { const b = btns[i]; if (!b) return; cur = b.getAttribute('data-tab'); paint(); if (focus) b.focus(); if (x.onChange) x.onChange(cur); };
        root.addEventListener('click', ev => { const b = hit(ev, '.hd-f-tab'); if (b && btns.includes(b)) choose(btns.indexOf(b), false); });
        root.addEventListener('keydown', ev => { if (!HORIZONTAL.includes(ev.key)) return; const at = btns.findIndex(b => b === ev.target), n = roving(ev.key, Math.max(0, at), btns.length); if (n === null || at < 0) return; ev.preventDefault(); choose(n, true); });
        paint(); return { el: root, set(v) { cur = v; paint(); }, idOf, focus() { const b = btns.find(y => y.getAttribute('tabindex') === '0') || btns[0]; if (b) b.focus(); } };
    }
    function button(o) {
        const x = o || {}, iconOnly = !!x.iconOnly, b = el('button', { type: 'button', class: 'hd-btn', 'data-tone': x.tone || 'default', 'data-hd-icon-only': iconOnly ? 'true' : null, 'aria-label': iconOnly ? x.label : null, title: iconOnly ? (x.title || x.label) : x.title },
            [x.icon ? ico(x.icon) : null, iconOnly ? null : el('span', { text: x.label || '' })]);
        b.addEventListener('click', ev => { if (b.getAttribute('aria-disabled') === 'true') return; if (x.onClick) x.onClick(ev); });
        return { el: b, set(v) { b.setAttribute('aria-disabled', v === false ? 'true' : 'false'); }, focus() { b.focus(); } };
    }
    function section(title, children) {
        const id = uid('hd-sec'), s = el('section', { class: 'hd-f-sec', 'aria-labelledby': id }, [el('h3', { class: 'hd-f-sec-title', id, text: title })]);
        for (const c of [].concat(children || [])) if (c) s.appendChild(c.el || c); return s;
    }
    // Searchable icon list: one aria-pressed button per E.icons.choices() name, labelled with its translated name (homedash.icon.<name>), the search
    // matches that label and the id. Arrow keys move a roving tabindex over the visible buttons (6 columns).
    function iconGrid(o) {
        const x = o || {}, t = x.t, label = n => (n ? t('homedash.icon.' + n) : t('homedash.inspector.icon_auto')), names = (x.allowAuto ? [''] : []).concat(E.icons.choices());
        const root = el('div', { class: 'hd-icongrid' }), list = el('div', { class: 'hd-icongrid-list', role: 'group', 'aria-label': x.label }), none = el('p', { class: 'hd-f-help', hidden: true, text: t('homedash.inspector.icon_none') });
        const search = text({ key: 'icon-search', search: true, max: 40, label: t('homedash.inspector.icon_search'), placeholder: t('homedash.inspector.icon_search'), onInput: v => filter(v) });
        const opts = names.map(n => { const b = el('button', { type: 'button', class: 'hd-icongrid-opt', 'aria-pressed': 'false', title: label(n), 'aria-label': label(n), 'data-icon': n, tabindex: '-1' }, ico(n || 'mode-auto')); list.appendChild(b); return b; });
        const keyOf = b => fold(label(b.getAttribute('data-icon')) + ' ' + b.getAttribute('data-icon')), visible = () => opts.filter(b => !b.hasAttribute('hidden'));
        let cur = x.value || '';
        const paint = () => { opts.forEach(b => b.setAttribute('aria-pressed', b.getAttribute('data-icon') === cur ? 'true' : 'false')); const v = visible(), on = v.find(b => b.getAttribute('aria-pressed') === 'true') || v[0];
            opts.forEach(b => b.setAttribute('tabindex', b === on ? '0' : '-1')); };
        function filter(q) { const k = fold(q).trim().split(/\s+/).filter(Boolean); opts.forEach(b => setAttr(b, 'hidden', k.every(w => keyOf(b).includes(w)) ? null : true)); setAttr(none, 'hidden', visible().length ? true : null); paint(); }
        list.addEventListener('click', ev => { const b = hit(ev, '.hd-icongrid-opt'); if (!b) return; cur = b.getAttribute('data-icon'); paint(); if (x.onPick) x.onPick(cur); });
        list.addEventListener('keydown', ev => { const v = visible(), at = v.findIndex(b => b === ev.target); if (at < 0) return; if (ev.key === 'Enter') { ev.preventDefault(); cur = v[at].getAttribute('data-icon'); paint(); if (x.onPick) x.onPick(cur); return; }
            const n = gridMove(ev.key, at, [v.length], 6); if (n === null) return; ev.preventDefault(); v[n].focus(); opts.forEach(b => b.setAttribute('tabindex', b === v[n] ? '0' : '-1')); if (typeof v[n].scrollIntoView === 'function') v[n].scrollIntoView({ block: 'nearest' }); });
        root.append(search.el, list, none); paint();
        return { el: root, set(v) { cur = v || ''; paint(); }, focus() { search.focus(); }, search };
    }
    // Opens a form in the designer's or app's E.sheet with no built-in actions and adds its own .hd-dlg-btn buttons (E.sheet does not auto-close
    // them), so a form can refuse to close while invalid: onAction(id) === false keeps it open.
    function formSheet(sheet, o) {
        const done = sheet.open({ title: o.title, text: [], actions: [], kind: o.kind || 'form' });
        const body = sheet.el.querySelector('.hd-sheet-body'), foot = sheet.el.querySelector('.hd-sheet-foot');
        body.appendChild(o.content);
        for (const a of o.actions || []) foot.appendChild(el('button', { type: 'button', class: 'hd-dlg-btn', 'data-action': a.id, 'data-tone': a.tone || 'default', text: a.label }));
        const onClick = ev => { const b = hit(ev, '.hd-dlg-btn'); if (!b) return; const id = b.getAttribute('data-action'); if (!o.onAction || o.onAction(id) !== false) sheet.close(id); };
        foot.addEventListener('click', onClick);
        const first = typeof o.first === 'function' ? o.first() : o.first; if (first && typeof first.focus === 'function') first.focus();
        return done.then(v => { foot.removeEventListener('click', onClick); return v; });
    }
    E.form = Object.freeze({ roving, gridMove, runes, fold, textIssue, field, text, number, select, toggle, segmented, colour, range, tabs, button, section, iconGrid, formSheet });
})();
