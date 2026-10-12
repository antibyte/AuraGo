// CYD Creator designer inspector fields: labelled controls per prop kind (text, number, optional
// number, checkbox, select, colour with "theme" reset, list, asset with upload) that call back on
// every edit, plus the browser-side SVG → PNG rasteriser used before inst.uploadAsset. Every control
// carries data-cc-field and data-cc-focus = its key (focus survives re-renders, tests find it).
// Values are passed on only in the form internal/cydcreator/validate.go accepts: numbers as integers
// clamped to their range, colours as #RRGGBB. A field with opts.check shows the message for a value
// it refuses, keeps that value in the field (opts.onRefuse, opts.draft brings it back after a
// re-render) and does not pass it on; an accepted value calls opts.onAccept and onChange.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const h = (...args) => P.render.h(...args);
    const HEX = /^#[0-9A-Fa-f]{6}$/; // colorPattern, internal/cydcreator/validate.go
    const NUL = String.fromCharCode(0);

    // clampInt returns v as an integer within [min, max], or null for an empty or non-numeric value.
    function clampInt(v, min, max) {
        if (v === null || v === undefined || typeof v === 'boolean' || (typeof v === 'string' && !v.trim())) return null;
        let n = Math.round(Number(v));
        if (!Number.isFinite(n)) return null;
        if (typeof min === 'number') n = Math.max(min, n);
        if (typeof max === 'number') n = Math.min(max, n);
        return n === 0 ? 0 : n;
    }

    // utf8Bytes is the UTF-8 length of s: the server counts text limits in bytes (a lone surrogate
    // arrives as U+FFFD, three bytes).
    function utf8Bytes(s) {
        let n = 0;
        for (const ch of String(s)) {
            const c = ch.codePointAt(0);
            n += c < 0x80 ? 1 : c < 0x800 ? 2 : c < 0x10000 ? 3 : 4;
        }
        return n;
    }

    // textIssue mirrors validate.go textProblem: PROP_INVALID for a NUL character, TEXT_TOO_LONG
    // above limit bytes, '' for a usable text.
    function textIssue(s, limit) {
        const v = String(s);
        if (v.includes(NUL)) return 'PROP_INVALID';
        return utf8Bytes(v) > limit ? 'TEXT_TOO_LONG' : '';
    }

    // listItems splits a list editor's text into items: one per line, trimmed; with rowBreaks an empty
    // line is a "\n" row break (keypad keys), else empty lines are dropped.
    function listItems(raw, rowBreaks) {
        return String(raw).split('\n').map(s => s.trim()).map(s => (rowBreaks && !s ? '\n' : s)).filter(s => rowBreaks || s);
    }

    // create binds the builders to a translator t(key, vars) with t.or.
    function create(t) {
        const wrap = (label, control, ...extra) => h('label', { class: 'cc-field' }, h('span', { class: 'cc-field-label' }, label), control, extra);
        const text = s => document.createTextNode(s);

        // note is the line under a control: the static hint, or the message o.check returns for the
        // shown value. update(input, raw) checks an edit, shows the outcome and returns the message
        // ('' = accepted).
        function note(o, shown) {
            if (!o.check && !o.hint) return { id: null, node: null, invalid: false, update: () => '' };
            const id = P.util.uid('cc_note');
            let el = null;
            const first = o.check ? o.check(shown) : '';
            return {
                id,
                invalid: !!first,
                node: h('span', { class: first ? 'cc-field-error' : 'cc-field-hint', id, 'aria-live': o.check ? 'polite' : null, ref: x => { el = x; } }, first || o.hint || ''),
                update(input, raw) {
                    const msg = o.check ? o.check(raw) : '';
                    if (el) { el.replaceChildren(text(msg || o.hint || '')); el.setAttribute('class', msg ? 'cc-field-error' : 'cc-field-hint'); }
                    input.setAttribute('aria-invalid', msg ? 'true' : 'false');
                    return msg;
                }
            };
        }
        // edited passes a typed value on when the field's check accepts it.
        function edited(o, n, onChange, parse) {
            return e => {
                const raw = e.target.value;
                if (n.update(e.target, raw)) { if (o.onRefuse) o.onRefuse(raw); return; }
                if (o.onAccept) o.onAccept();
                onChange(parse ? parse(raw) : raw);
            };
        }
        const shownText = (o, value) => (o.draft !== undefined ? String(o.draft) : String(value == null ? '' : value));

        // text: one line. opts {maxlength, list, hint, check, draft, onRefuse, onAccept}.
        function textField(label, key, value, opts, onChange) {
            const o = opts || {};
            const shown = shownText(o, value);
            const n = note(o, shown);
            return wrap(label, h('input', {
                type: 'text', value: shown, maxlength: o.maxlength ? String(o.maxlength) : null, list: o.list || null, autocomplete: 'off', spellcheck: 'false',
                'data-cc-field': key, 'data-cc-focus': key, 'aria-describedby': n.id, 'aria-invalid': n.invalid ? 'true' : null,
                on: { input: edited(o, n, onChange) }
            }), n.node);
        }

        // textarea: several lines; opts as text plus placeholder and counter(raw) → text shown below.
        function textarea(label, key, value, opts, onChange) {
            const o = opts || {};
            const shown = shownText(o, value);
            const n = note(o, shown);
            const countId = o.counter ? P.util.uid('cc_count') : null;
            let counter = null;
            const input = edited(o, n, onChange);
            return wrap(label, h('textarea', {
                rows: '3', value: shown, maxlength: o.maxlength ? String(o.maxlength) : null, placeholder: o.placeholder || null,
                'data-cc-field': key, 'data-cc-focus': key, 'aria-describedby': [n.id, countId].filter(Boolean).join(' ') || null, 'aria-invalid': n.invalid ? 'true' : null,
                on: { input: e => { if (counter) counter.replaceChildren(text(o.counter(e.target.value))); input(e); } }
            }), n.node, o.counter ? h('span', { class: 'cc-field-count', id: countId, ref: x => { counter = x; } }, o.counter(shown)) : null);
        }

        // number: an integer clamped to [opts.min, opts.max] on every keystroke; leaving the field shows
        // the stored number again (an empty or partial entry stores nothing).
        function number(label, key, value, opts, onChange) {
            const o = opts || {};
            let last = typeof value === 'number' && Number.isFinite(value) ? String(value) : '';
            return wrap(label, h('input', {
                type: 'number', step: '1', value: last, min: typeof o.min === 'number' ? String(o.min) : null, max: typeof o.max === 'number' ? String(o.max) : null,
                'data-cc-field': key, 'data-cc-focus': key,
                on: {
                    input: e => { const n = clampInt(e.target.value, o.min, o.max); if (n === null) return; last = String(n); onChange(n); },
                    change: e => { const n = clampInt(e.target.value, o.min, o.max); e.target.value = n === null ? last : String(n); }
                }
            }));
        }

        // optionalNumber is empty for "theme default"; clearing it calls onChange(null).
        function optionalNumber(label, key, value, opts, placeholder, onChange) {
            const o = opts || {};
            let last = typeof value === 'number' && Number.isFinite(value) ? String(value) : '';
            return wrap(label, h('input', {
                type: 'number', step: '1', value: last, placeholder, min: String(o.min), max: String(o.max), 'data-cc-field': key, 'data-cc-focus': key,
                on: {
                    input: e => {
                        if (!String(e.target.value).trim()) { last = ''; onChange(null); return; }
                        const n = clampInt(e.target.value, o.min, o.max);
                        if (n === null) return;
                        last = String(n);
                        onChange(n);
                    },
                    change: e => { const n = clampInt(e.target.value, o.min, o.max); e.target.value = n === null ? last : String(n); }
                }
            }));
        }

        function check(label, key, value, onChange, opts) {
            const o = opts || {};
            return h('label', { class: 'cc-check' + (o.disabled ? ' is-disabled' : '') },
                h('input', { type: 'checkbox', checked: !!value, disabled: !!o.disabled, 'data-cc-field': key, 'data-cc-focus': key, on: { change: e => onChange(!!e.target.checked) } }),
                h('span', {}, label),
                o.hint ? h('span', { class: 'cc-field-hint' }, o.hint) : null);
        }

        // select: options are [value, label, disabled?]. A stored value that is not among them (an
        // invalid layout) is listed as itself, so the field shows what the layout holds.
        function select(label, key, options, value, onChange) {
            const v0 = String(value == null ? '' : value);
            const list = options.some(([v]) => v === v0) ? options : options.concat([[v0, v0 + ' ?', true]]);
            return wrap(label, h('select', { 'data-cc-field': key, 'data-cc-focus': key, on: { change: e => onChange(e.target.value) } },
                list.map(([v, l, off]) => h('option', { value: v, selected: v === v0, disabled: !!off && v !== v0 }, l))));
        }

        // color: the picker passes #RRGGBB on (upper case, like the server's examples); "Use theme"
        // calls onReset. value '' means the theme's colour (fallback is only shown).
        function color(label, key, value, fallback, onPick, onReset) {
            const id = P.util.uid('cc_color');
            const own = typeof value === 'string' && HEX.test(value);
            const shown = own ? value : (typeof fallback === 'string' && HEX.test(fallback) ? fallback : '#000000');
            let code = null;
            let reset = null;
            return h('div', { class: 'cc-field' }, h('span', { class: 'cc-field-label', id }, label),
                h('div', { class: 'cc-color-row' },
                    h('input', {
                        type: 'color', value: shown.toLowerCase(), 'aria-labelledby': id, 'data-cc-field': key, 'data-cc-focus': key,
                        on: {
                            input: e => {
                                const v = String(e.target.value || '');
                                if (!HEX.test(v)) return;
                                const up = v.toUpperCase();
                                if (code) code.replaceChildren(text(up));
                                if (reset) reset.setAttribute('aria-pressed', 'false');
                                onPick(up);
                            }
                        }
                    }),
                    h('code', { class: 'cc-color-value', ref: x => { code = x; } }, value ? String(value) : t.or('cyd_creator.designer.theme_value', 'Theme')),
                    h('button', { type: 'button', class: 'cc-chip', 'aria-pressed': value ? 'false' : 'true', 'data-cc-focus': key + ':reset', ref: x => { reset = x; }, on: { click: onReset } },
                        t.or('cyd_creator.designer.use_theme', 'Use theme'))));
        }

        // list edits one item per line; with rowBreaks an empty line stands for a "\n" row break
        // (keypad keys). opts.check receives the parsed items.
        function list(label, key, items, rowBreaks, opts, onChange) {
            const o = opts || {};
            const shown = o.draft !== undefined ? String(o.draft) : items.map(i => (rowBreaks && i === '\n' ? '' : String(i))).join('\n');
            const hint = rowBreaks ? t.or('cyd_creator.designer.list_hint_keys', 'One key per line; an empty line starts a new row.') : t.or('cyd_creator.designer.list_hint', 'One entry per line.');
            const n = note(Object.assign({}, o, { hint, check: o.check ? raw => o.check(listItems(raw, rowBreaks)) : null }), shown);
            return wrap(label, h('textarea', {
                rows: String(Math.min(8, Math.max(3, items.length + 1))), value: shown, spellcheck: 'false',
                'data-cc-field': key, 'data-cc-focus': key, 'aria-describedby': n.id, 'aria-invalid': n.invalid ? 'true' : null,
                on: { input: edited(o, n, onChange, raw => listItems(raw, rowBreaks)) }
            }), n.node);
        }

        function asset(d, label, key, value, uploading, onSelect, onUpload) {
            const assets = Array.isArray(d.project().assets) ? d.project().assets : [];
            const base = file => String(file || '').split('/').pop();
            return h('div', { class: 'cc-field' },
                select(label, key, [['', t.or('cyd_creator.designer.asset_none', 'No image')]].concat(assets.map(a => [String(a.id), base(a.file) + ' (' + a.w + '×' + a.h + ')'])), value, onSelect),
                h('div', { class: 'cc-asset-row' },
                    h('label', { class: 'cc-chip cc-upload' + (uploading ? ' is-busy' : '') }, P.render.icon('upload'), h('span', {}, t.or('cyd_creator.designer.asset_upload', 'Upload image')),
                        h('input', {
                            type: 'file', class: 'cc-sr-only', disabled: !!uploading, accept: '.png,.jpg,.jpeg,.svg,image/png,image/jpeg,image/svg+xml',
                            'data-cc-field': key + ':upload', 'data-cc-focus': key + ':upload',
                            on: { change: e => { const f = e.target.files && e.target.files[0]; e.target.value = ''; if (f) onUpload(f); } }
                        })),
                    uploading
                        ? h('span', { class: 'cc-field-hint', role: 'status' }, t.or('cyd_creator.designer.asset_uploading', 'Uploading…'))
                        : h('span', { class: 'cc-field-hint' }, t.or('cyd_creator.designer.asset_hint', 'PNG or JPEG; SVG becomes a PNG in the widget’s size.'))));
        }

        return { text: textField, textarea, number, optionalNumber, check, select, color, list, asset, isSVG, rasterizeSVG };
    }

    function isSVG(file) { return /svg/i.test(file.type || '') || /\.svg$/i.test(file.name || ''); }

    // rasterizeSVG draws an SVG file at w×h (else its own size, else 64×64) and returns a PNG File.
    async function rasterizeSVG(file, w, h2) {
        const source = await file.text();
        const url = URL.createObjectURL(new Blob([source], { type: 'image/svg+xml' }));
        try {
            const img = new Image();
            img.decoding = 'async';
            img.src = url;
            await img.decode();
            const width = Math.max(1, Math.round(w || img.naturalWidth || 64));
            const height = Math.max(1, Math.round(h2 || img.naturalHeight || 64));
            const canvas = document.createElement('canvas');
            canvas.width = width;
            canvas.height = height;
            canvas.getContext('2d').drawImage(img, 0, 0, width, height);
            const blob = await new Promise((resolve, reject) => canvas.toBlob(b => (b ? resolve(b) : reject(new Error('png encode failed'))), 'image/png'));
            return new File([blob], String(file.name || 'image').replace(/\.svg$/i, '') + '.png', { type: 'image/png' });
        } finally {
            URL.revokeObjectURL(url);
        }
    }

    P.designerFields = { create, clampInt, utf8Bytes, textIssue, listItems, isSVG, rasterizeSVG, HEX };
})();
