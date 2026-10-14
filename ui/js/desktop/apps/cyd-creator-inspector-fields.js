// CYD Creator designer inspector fields: labelled controls per prop kind (text, number, optional
// number, checkbox, select, colour with "theme" reset, list, asset with upload) that call back on
// every edit, plus the browser-side upload checks and SVG → PNG rasteriser used before
// inst.uploadAsset. Every control carries data-cc-field and data-cc-focus = its key (focus survives
// re-renders, tests find it). A <label for> names each control; hint, error and counter lines follow
// outside the label and are linked with aria-describedby (read once). Values are passed on only in
// the form internal/cydcreator/validate.go accepts: numbers as integers clamped to their range (a
// clamp is announced), colours as #RRGGBB. A field with opts.check shows the message for a value it
// refuses, keeps that value in the field (opts.onRefuse, opts.draft brings it back after a
// re-render) and does not pass it on; an accepted value calls opts.onAccept and onChange.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const h = (...args) => P.render.h(...args);
    const HEX = /^#[0-9A-Fa-f]{6}$/; // colorPattern, internal/cydcreator/validate.go
    const NUL = String.fromCharCode(0);
    const MAX_UPLOAD_BYTES = 2 << 20; // MaxAssetUploadBytes, internal/cydcreator/assets.go
    const PNG_MAGIC = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];

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

    // listItems splits a list editor's text into items, one per line, trimmed. Lists are addressed
    // by index (table cells by row and column, tabs by their children's tab, options by selected,
    // series, items and buttons in generated code), so an empty line stays an empty item; only
    // trailing empty lines (the line being started) are dropped. With rowBreaks (keypad keys) an
    // empty line is a "\n" row break: repeated breaks merge, leading and trailing ones are dropped
    // (each would be an empty LVGL row).
    function listItems(raw, rowBreaks) {
        const lines = String(raw).split('\n').map(s => s.trim());
        if (!rowBreaks) {
            while (lines.length && !lines[lines.length - 1]) lines.pop();
            return lines;
        }
        const out = [];
        lines.forEach(s => {
            if (s) out.push(s);
            else if (out.length && out[out.length - 1] !== '\n') out.push('\n');
        });
        if (out[out.length - 1] === '\n') out.pop();
        return out;
    }

    // create binds the builders to a translator t(key, vars) with t.or; env.announce(text) speaks
    // through the designer's polite live region (number clamps).
    function create(t, env) {
        const announce = env && typeof env.announce === 'function' ? env.announce : () => {};
        const text = s => document.createTextNode(s);
        const field = (label, id, control, ...extra) => h('div', { class: 'cydc-field' }, h('label', { class: 'cydc-field-label', for: id }, label), control, extra);

        // note is the line under a control: the static hint, or the message o.check returns for the
        // shown value. show(input, msg) shows a message (or the hint again); update(input, raw)
        // checks an edit, shows the outcome and returns the message ('' = accepted).
        function note(o, shown) {
            if (!o.check && !o.hint && !o.live) return { id: null, node: null, invalid: false, show: () => {}, update: () => '' };
            const id = P.util.uid('cc_note');
            let el = null;
            const first = o.check ? o.check(shown) : '';
            const show = (input, msg) => {
                if (el) { el.replaceChildren(text(msg || o.hint || '')); el.setAttribute('class', msg ? 'cydc-field-error' : 'cydc-field-hint'); }
                input.setAttribute('aria-invalid', msg ? 'true' : 'false');
            };
            return {
                id,
                invalid: !!first,
                node: h('span', { class: first ? 'cydc-field-error' : 'cydc-field-hint', id, 'aria-live': o.check || o.live ? 'polite' : null, ref: x => { el = x; } }, first || o.hint || ''),
                show,
                update(input, raw) { const msg = o.check ? o.check(raw) : ''; show(input, msg); return msg; }
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
            const id = P.util.uid('cc_in');
            const shown = shownText(o, value);
            const n = note(o, shown);
            return field(label, id, h('input', {
                id, type: 'text', value: shown, maxlength: o.maxlength ? String(o.maxlength) : null, list: o.list || null, autocomplete: 'off', spellcheck: 'false',
                'data-cc-field': key, 'data-cc-focus': key, 'aria-describedby': n.id, 'aria-invalid': n.invalid ? 'true' : null,
                on: { input: edited(o, n, onChange) }
            }), n.node);
        }

        // textarea: several lines; opts as text plus placeholder and counter(raw) → text shown below.
        function textarea(label, key, value, opts, onChange) {
            const o = opts || {};
            const id = P.util.uid('cc_in');
            const shown = shownText(o, value);
            const n = note(o, shown);
            const countId = o.counter ? P.util.uid('cc_count') : null;
            let counter = null;
            const input = edited(o, n, onChange);
            return field(label, id, h('textarea', {
                id, rows: '3', value: shown, maxlength: o.maxlength ? String(o.maxlength) : null, placeholder: o.placeholder || null,
                'data-cc-field': key, 'data-cc-focus': key, 'aria-describedby': [n.id, countId].filter(Boolean).join(' ') || null, 'aria-invalid': n.invalid ? 'true' : null,
                on: { input: e => { if (counter) counter.replaceChildren(text(o.counter(e.target.value))); input(e); } }
            }), n.node, o.counter ? h('span', { class: 'cydc-field-count', id: countId, ref: x => { counter = x; } }, o.counter(shown)) : null);
        }

        // wholeNumber: an integer clamped to [o.min, o.max] on every keystroke (a clamp is announced);
        // an entry the browser cannot read ("-", "5e") stores nothing and says so; leaving the field
        // shows the stored number again. An empty entry calls onEmpty (optional numbers) or nothing.
        function wholeNumber(label, key, value, o, onValue, onEmpty) {
            const id = P.util.uid('cc_in');
            const n = note({ live: true }, '');
            const wrong = () => t.or('cyd_creator.designer.number_invalid', 'Enter a whole number.');
            const unreadable = el => !!(el.validity && el.validity.badInput);
            let last = typeof value === 'number' && Number.isFinite(value) ? String(value) : '';
            let spoken = null;
            return field(label, id, h('input', {
                id, type: 'number', step: '1', value: last, placeholder: o.placeholder || null,
                min: typeof o.min === 'number' ? String(o.min) : null, max: typeof o.max === 'number' ? String(o.max) : null,
                'data-cc-field': key, 'data-cc-focus': key, 'aria-describedby': n.id,
                on: {
                    input: e => {
                        const el = e.target;
                        const raw = String(el.value == null ? '' : el.value);
                        if (unreadable(el)) { n.show(el, wrong()); return; }
                        if (!raw.trim()) { n.show(el, ''); if (onEmpty) { last = ''; onEmpty(); } return; }
                        const v = clampInt(raw, o.min, o.max);
                        if (v === null) { n.show(el, wrong()); return; }
                        n.show(el, '');
                        const clamped = Math.round(Number(raw)) !== v;
                        if (clamped && spoken !== v) announce(t.or('cyd_creator.designer.value_clamped', 'Allowed: {min} to {max}. Stored: {value}.', { min: o.min, max: o.max, value: v }));
                        spoken = clamped ? v : null;
                        last = String(v);
                        onValue(v);
                    },
                    change: e => {
                        const v = unreadable(e.target) ? null : clampInt(e.target.value, o.min, o.max);
                        e.target.value = v === null ? last : String(v);
                        n.show(e.target, '');
                    }
                }
            }), n.node);
        }
        const number = (label, key, value, opts, onChange) => wholeNumber(label, key, value, opts || {}, onChange, null);
        // optionalNumber is empty for "theme default"; clearing it calls onChange(null).
        const optionalNumber = (label, key, value, opts, placeholder, onChange) => wholeNumber(label, key, value, Object.assign({}, opts, { placeholder }), onChange, () => onChange(null));

        function check(label, key, value, onChange, opts) {
            const o = opts || {};
            const hintId = o.hint ? P.util.uid('cc_hint') : null;
            return h('div', { class: 'cydc-check-field' },
                h('label', { class: 'cydc-check' + (o.disabled ? ' is-disabled' : '') },
                    h('input', { type: 'checkbox', checked: !!value, disabled: !!o.disabled, 'aria-describedby': hintId, 'data-cc-field': key, 'data-cc-focus': key, on: { change: e => onChange(!!e.target.checked) } }),
                    h('span', {}, label)),
                o.hint ? h('span', { class: 'cydc-field-hint', id: hintId }, o.hint) : null);
        }

        // select: options are [value, label, disabled?]. A stored value that is not among them (an
        // invalid layout) stays listed and selected, labelled missing (else as itself with " ?").
        function select(label, key, options, value, onChange, missing) {
            const id = P.util.uid('cc_in');
            const v0 = String(value == null ? '' : value);
            const list = options.some(([v]) => v === v0) ? options : options.concat([[v0, missing || v0 + ' ?', true]]);
            return field(label, id, h('select', { id, 'data-cc-field': key, 'data-cc-focus': key, on: { change: e => onChange(e.target.value) } },
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
            return h('div', { class: 'cydc-field' }, h('label', { class: 'cydc-field-label', for: id }, label),
                h('div', { class: 'cydc-color-row' },
                    h('input', {
                        id, type: 'color', value: shown.toLowerCase(), 'data-cc-field': key, 'data-cc-focus': key,
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
                    h('code', { class: 'cydc-color-value', ref: x => { code = x; } }, value ? String(value) : t.or('cyd_creator.designer.theme_value', 'Theme')),
                    h('button', { type: 'button', class: 'cydc-chip', 'aria-pressed': value ? 'false' : 'true', 'data-cc-focus': key + ':reset', ref: x => { reset = x; }, on: { click: onReset } },
                        t.or('cyd_creator.designer.use_theme', 'Use theme'))));
        }

        // list edits one item per line (listItems); with rowBreaks an empty line stands for a "\n"
        // row break (keypad keys). opts.check receives the parsed items.
        function list(label, key, items, rowBreaks, opts, onChange) {
            const o = opts || {};
            const id = P.util.uid('cc_in');
            const shown = o.draft !== undefined ? String(o.draft) : items.map(i => (rowBreaks && i === '\n' ? '' : String(i))).join('\n');
            const hint = rowBreaks ? t.or('cyd_creator.designer.list_hint_keys', 'One key per line; an empty line starts a new row.') : t.or('cyd_creator.designer.list_hint', 'One entry per line.');
            const n = note(Object.assign({}, o, { hint, check: o.check ? raw => o.check(listItems(raw, rowBreaks)) : null }), shown);
            return field(label, id, h('textarea', {
                id, rows: String(Math.min(8, Math.max(3, items.length + 1))), value: shown, spellcheck: 'false',
                'data-cc-field': key, 'data-cc-focus': key, 'aria-describedby': n.id, 'aria-invalid': n.invalid ? 'true' : null,
                on: { input: edited(o, n, onChange, raw => listItems(raw, rowBreaks)) }
            }), n.node);
        }

        function asset(d, label, key, value, uploading, onSelect, onUpload) {
            const assets = Array.isArray(d.project().assets) ? d.project().assets : [];
            const base = file => String(file || '').split('/').pop();
            const statusId = P.util.uid('cc_hint');
            return h('div', { class: 'cydc-field' },
                select(label, key, [['', t.or('cyd_creator.designer.asset_none', 'No image')]].concat(assets.map(a => [String(a.id), base(a.file) + ' (' + a.w + '×' + a.h + ')'])), value, onSelect),
                h('div', { class: 'cydc-asset-row' },
                    h('label', { class: 'cydc-chip cydc-upload' + (uploading ? ' is-busy' : '') }, P.render.icon('upload'), h('span', {}, t.or('cyd_creator.designer.asset_upload', 'Upload image')),
                        h('input', {
                            type: 'file', class: 'cydc-sr-only', disabled: !!uploading, accept: '.png,.jpg,.jpeg,.svg,image/png,image/jpeg,image/svg+xml', 'aria-describedby': statusId,
                            'data-cc-field': key + ':upload', 'data-cc-focus': key + ':upload',
                            on: { change: e => { const f = e.target.files && e.target.files[0]; e.target.value = ''; if (f) onUpload(f); } }
                        })),
                    uploading
                        ? h('span', { class: 'cydc-field-hint', id: statusId, role: 'status' }, t.or('cyd_creator.designer.asset_uploading', 'Uploading…'))
                        : h('span', { class: 'cydc-field-hint', id: statusId }, t.or('cyd_creator.designer.asset_hint', 'PNG or JPEG; SVG becomes a PNG in the widget’s size.'))));
        }

        return { text: textField, textarea, number, optionalNumber, check, select, color, list, asset, isSVG, rasterizeSVG };
    }

    function isSVG(file) { return /svg/i.test(file.type || '') || /\.svg$/i.test(file.name || ''); }

    // imageKind mirrors the server's upload check (assets.go assetExtension: PNG and JPEG by their
    // magic bytes) plus SVG, which the browser rasterises to PNG first; '' = not uploadable.
    async function imageKind(file) {
        if (!file) return '';
        if (isSVG(file)) return 'svg';
        try {
            const head = new Uint8Array(await file.slice(0, 8).arrayBuffer());
            if (PNG_MAGIC.every((b, i) => head[i] === b)) return 'png';
            if (head[0] === 0xff && head[1] === 0xd8 && head[2] === 0xff) return 'jpeg';
        } catch (err) { /* unreadable file */ }
        return '';
    }

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

    P.designerFields = { create, clampInt, utf8Bytes, textIssue, listItems, isSVG, imageKind, rasterizeSVG, HEX, MAX_UPLOAD_BYTES };
})();
