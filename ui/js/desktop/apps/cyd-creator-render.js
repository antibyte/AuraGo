// CYD Creator designer renderers: a tiny virtual-node builder (h → dom/html) used by every designer
// module, and HTML/CSS previews for all 26 widget types of the catalog. Colours, radius, border width,
// font and button style come only from the theme sent by the API plus the widget's style overrides;
// this file holds no theme values. Each preview follows the generator (generate_widgets.go,
// generate_theme.go) and the LVGL 9 default theme; the only fixed colours are the ones the device
// draws regardless of the theme (QR code black on white, the checkbox tick). Close, not bit-exact.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const SVG_NS = 'http://www.w3.org/2000/svg';
    const SVG_TAGS = new Set(['svg', 'g', 'path', 'circle', 'line', 'polyline', 'rect', 'text']);
    const PROP_KEYS = new Set(['value', 'checked', 'disabled', 'selected', 'hidden', 'indeterminate']);
    const FAMILIES = {
        montserrat: "'CYD Montserrat', 'Montserrat', sans-serif",
        inter: "'Inter', sans-serif",
        jetbrains_mono: "'CYD JetBrains Mono', 'JetBrains Mono', monospace"
    };
    // Catalog FontSizes; the statusbar uses the theme's small role (next size below the base).
    const FONT_SIZES = [12, 14, 16, 20, 24, 28, 32, 40, 48];
    // LVGL 9 default-theme metrics at LV_DPI_DEF 130 on displays up to 320 px (most CYD boards;
    // bigger panels get a few pixels more): pads, arc width, knob pads, checkbox marker pad, scale
    // tick length, chart point radius and series gap, active-tab border and msgbox bar height (dpi/3).
    const LV = { PAD_DEF: 13, PAD_SMALL: 8, ARC_W: 12, KNOB_PAD: 5, SWITCH_INSET: 3, MARKER_PAD: 2, TICK: 5, POINT_R: 3, SER_GAP: 2, TAB_BORDER: 4, BAR_H: 43 };
    const LED_MIN = 80; // LV_LED_BRIGHT_MIN
    // Fixed device colours: lv_qrcode dark/light set by the generator, lv_color_white() checkbox tick.
    const QR_DARK = 'black', QR_LIGHT = 'white', LV_WHITE = 'white';
    // Unicode stand-ins for the LVGL symbol font (FontAwesome subset) in the preview.
    const SYMBOLS = {
        audio: '♪', video: '▣', list: '☰', ok: '✓', close: '✕', power: '⏻', settings: '⚙', home: '⌂', download: '⤓',
        drive: '⛁', refresh: '↻', mute: '∅', volume_mid: '◔', volume_max: '◕', image: '▨', tint: '◐', prev: '⏮',
        play: '▶', pause: '⏸', stop: '■', next: '⏭', eject: '⏏', left: '◀', right: '▶', plus: '+', minus: '−',
        eye_open: '◉', eye_close: '◎', warning: '⚠', shuffle: '⤮', up: '▲', down: '▼', loop: '⟲', directory: '▭',
        upload: '⤒', call: '✆', cut: '✂', copy: '❐', save: '▤', bars: '≡', envelope: '✉', charge: 'ϟ', paste: '▥',
        bell: '⍾', keyboard: '⌨', gps: '⌖', file: '▯', wifi: '◠', battery_full: '▮▮▮▮', battery_3: '▮▮▮▯',
        battery_2: '▮▮▯▯', battery_1: '▮▯▯▯', battery_empty: '▯▯▯▯', usb: 'ψ', bluetooth: 'ᛒ', trash: 'ⓧ',
        edit: '✎', backspace: '⌫', sd_card: '▙', new_line: '↵'
    };

    // ── virtual nodes ───────────────────────────────────────────────────────

    // h(tag, attrs, ...children): attrs may hold class, style (object, kebab-case keys), on
    // ({event: fn}), boolean attributes (false/null omit them) and plain string attributes.
    function h(tag, attrs, ...children) {
        const kids = [];
        (function add(list) {
            list.forEach(c => {
                if (c === null || c === undefined || c === false || c === true) return;
                if (Array.isArray(c)) add(c); else kids.push(typeof c === 'object' ? c : String(c));
            });
        })(children);
        return { tag, attrs: attrs || {}, children: kids };
    }

    function dom(node, ns) {
        if (typeof node === 'string') return document.createTextNode(node);
        const space = ns || (node.tag === 'svg' ? SVG_NS : null);
        const el = space ? document.createElementNS(space, node.tag) : document.createElement(node.tag);
        Object.keys(node.attrs).forEach(key => {
            const v = node.attrs[key];
            if (v === null || v === undefined || v === false) return;
            if (key === 'style') Object.keys(v).forEach(k => { if (v[k] !== null && v[k] !== undefined && v[k] !== '') el.style.setProperty(k, String(v[k])); });
            else if (key === 'on') Object.keys(v).forEach(type => el.addEventListener(type, v[type]));
            else if (key === 'ref') v(el);
            else if (PROP_KEYS.has(key) && !space) el[key] = v;
            else el.setAttribute(key === 'className' ? 'class' : key, v === true ? '' : String(v));
        });
        node.children.forEach(c => el.appendChild(dom(c, space && node.tag !== 'foreignObject' ? space : null)));
        return el;
    }

    // swap replaces host's children with nodes and keeps the focus (and text selection) on the
    // element with the same data-cc-focus key, so panels can re-render while being used.
    function swap(host, nodes) {
        const active = document.activeElement;
        const key = active && host.contains(active) ? active.getAttribute('data-cc-focus') : null;
        const range = key && typeof active.selectionStart === 'number' ? [active.selectionStart, active.selectionEnd] : null;
        host.replaceChildren(...[].concat(nodes).filter(Boolean).map(n => (typeof Node !== 'undefined' && n instanceof Node) ? n : dom(n)));
        if (!key) return;
        const next = Array.from(host.querySelectorAll('[data-cc-focus]')).find(el => el.getAttribute('data-cc-focus') === key);
        if (!next) return;
        next.focus({ preventScroll: true });
        if (range && typeof next.setSelectionRange === 'function') { try { next.setSelectionRange(range[0], range[1]); } catch (err) { /* type without selection */ } }
    }

    function esc(s) { return String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])); }

    // html serialises a node (tests, debugging); listeners and refs are dropped.
    function html(node) {
        if (typeof node === 'string') return esc(node);
        let out = '<' + node.tag;
        Object.keys(node.attrs).forEach(key => {
            const v = node.attrs[key];
            if (v === null || v === undefined || v === false || key === 'on' || key === 'ref') return;
            if (key === 'style') {
                const css = Object.keys(v).filter(k => v[k] !== null && v[k] !== undefined && v[k] !== '').map(k => k + ':' + v[k]).join(';');
                if (css) out += ' style="' + esc(css) + '"';
            } else out += v === true ? ' ' + key : ' ' + key + '="' + esc(v) + '"';
        });
        return out + '>' + node.children.map(html).join('') + '</' + node.tag + '>';
    }

    // ── theme helpers ───────────────────────────────────────────────────────

    function rgba(hex, alpha) {
        const m = /^#([0-9a-f]{6})$/i.exec(String(hex || ''));
        if (!m) return hex || 'transparent';
        const n = parseInt(m[1], 16);
        return 'rgba(' + (n >> 16 & 255) + ',' + (n >> 8 & 255) + ',' + (n & 255) + ',' + Math.round(alpha * 1000) / 1000 + ')';
    }

    // shade mixes a colour with black like lv_color_mix(color, black, mix), mix 0..255 (LED brightness).
    function shade(hex, mix) {
        const m = /^#([0-9a-f]{6})$/i.exec(String(hex || ''));
        if (!m) return hex || 'transparent';
        const n = parseInt(m[1], 16);
        const f = Math.max(0, Math.min(255, mix)) / 255;
        return 'rgb(' + [n >> 16 & 255, n >> 8 & 255, n & 255].map(v => Math.round(v * f)).join(',') + ')';
    }

    // colors resolves the theme tokens with the widget's style overrides. ownRadius is the radius
    // override alone (null without one): widgets the device does not round (label, image, tabview,
    // statusbar) fall back to 0 instead of the theme radius.
    function colors(theme, style) {
        const tk = (theme && theme.tokens) || {};
        const s = style || {};
        const set = v => v !== null && v !== undefined;
        const alpha = set(s.bg_opacity) ? Math.max(0, Math.min(255, s.bg_opacity)) / 255 : 1;
        return {
            tk,
            bg: s.bg_color ? rgba(s.bg_color, alpha) : null,
            alpha,
            opaSet: set(s.bg_opacity),
            text: s.text_color || tk.text,
            accent: s.accent_color || tk.accent,
            border: s.border_color || tk.border,
            bw: set(s.border_width) ? s.border_width : null,
            radius: set(s.radius) ? s.radius : (tk.radius || 0),
            ownRadius: set(s.radius) ? s.radius : null
        };
    }
    function fill(c, fallback) { return c.bg || (c.alpha < 1 ? rgba(fallback, c.alpha) : fallback); }
    function border(c, defaultWidth) {
        const width = c.bw === null ? defaultWidth : c.bw;
        return width > 0 ? width + 'px solid ' + c.border : 'none';
    }
    function own(v, fallback) { return v === null || v === undefined ? fallback : v; }
    function fontFamily(name) { return FAMILIES[name] || FAMILIES.montserrat; }
    function fontStyle(theme, widget, spec) {
        const pv = name => P.model.propValue(widget, spec, name);
        const family = (spec && P.model.propSpec(spec, 'font') && pv('font')) || (theme && theme.font) || 'montserrat';
        const size = (spec && P.model.propSpec(spec, 'size') && Number(pv('size'))) || (theme && theme.font_size) || 16;
        return { 'font-family': fontFamily(family), 'font-size': size + 'px', 'line-height': '1.2' };
    }
    // smallSize is the theme's small font role (themeFontRoles): the next catalog size below the base.
    function smallSize(base) {
        let small = base;
        FONT_SIZES.forEach(s => { if (s < base) small = s; });
        return small;
    }
    function symbolGlyph(name) { return name ? (SYMBOLS[name] || '◆') : ''; }
    function ratio(value, min, max) { return max > min ? Math.max(0, Math.min(1, (value - min) / (max - min))) : 0; }
    function list(v) { return Array.isArray(v) ? v.map(String) : []; }
    function box(extra) { return Object.assign({ position: 'absolute', left: '0', top: '0', width: '100%', height: '100%', 'box-sizing': 'border-box', overflow: 'hidden' }, extra); }
    function center(extra) { return Object.assign({ display: 'flex', 'align-items': 'center', 'justify-content': 'center' }, extra); }

    function arcPath(cx, cy, r, a0, a1) {
        const p = a => [cx + r * Math.cos(a * Math.PI / 180), cy + r * Math.sin(a * Math.PI / 180)].map(v => Math.round(v * 10) / 10);
        const s = p(a0), e = p(a1);
        return 'M' + s[0] + ' ' + s[1] + ' A' + r + ' ' + r + ' 0 ' + (a1 - a0 > 180 ? 1 : 0) + ' 1 ' + e[0] + ' ' + e[1];
    }

    // buttonFace styles a button like ui__style_button: filled = accent fill with on_accent text,
    // outline = accent text and accent border, bracket = accent text only. The accent override does
    // not reach buttons on the device (renderStyle has no button case); bg, text, border and radius do.
    // Without a bg_color override an outline/bracket button with bg_opacity shows the accent (LVGL's
    // primary button colour) at that opacity.
    function buttonFace(theme, c, style) {
        const kind = (theme && theme.button_style) || 'filled';
        const s = style || {};
        const tk = c.tk;
        const face = { 'border-radius': c.radius + 'px' };
        if (kind === 'filled') Object.assign(face, { background: fill(c, tk.accent), color: s.text_color ? c.text : tk.on_accent, border: border(c, 0) });
        else {
            Object.assign(face, {
                background: c.bg || (c.opaSet ? rgba(tk.accent, c.alpha) : 'transparent'),
                color: s.text_color ? c.text : tk.accent,
                border: border(Object.assign({}, c, { border: s.border_color || tk.accent }), kind === 'outline' ? Math.max(1, tk.border_width || 1) : 0)
            });
        }
        return { kind, face };
    }

    // buttonLabel is the label of buttonText(): optional symbol, optional text, "[ " … " ]" in bracket themes.
    function buttonLabel(kind, symbol, text) {
        const glyph = symbolGlyph(symbol);
        return h('span', { style: { 'white-space': 'pre' } },
            kind === 'bracket' ? '[ ' : null,
            glyph ? h('span', { class: 'cc-glyph' }, glyph) : null,
            glyph && text ? ' ' : null,
            text || null,
            kind === 'bracket' ? ' ]' : null);
    }

    // ── renderers (ctx: {w, spec, theme, c, pv, font, assetUrl, activeTab, screenTitle, settings}) ───

    const R = {
        label(x) {
            const mode = x.pv('long_mode');
            const nowrap = mode === 'dots' || mode === 'scroll';
            return h('div', { style: box(Object.assign({}, x.font, { color: x.c.text, background: x.c.bg || 'transparent', border: border(x.c, 0), 'border-radius': own(x.c.ownRadius, 0) + 'px', 'text-align': x.pv('align') || 'left', 'white-space': nowrap ? 'nowrap' : 'pre-wrap', 'text-overflow': mode === 'dots' ? 'ellipsis' : 'clip', 'overflow-wrap': 'anywhere' })) }, String(x.pv('text') || ''));
        },
        button(x) {
            const b = buttonFace(x.theme, x.c, x.w.style);
            return h('div', { style: box(center(Object.assign({}, x.font, b.face, { padding: '0 6px', 'white-space': 'nowrap' }))) }, buttonLabel(b.kind, x.pv('symbol'), String(x.pv('text') || '')));
        },
        image(x) {
            const id = x.pv('asset');
            if (id && typeof x.assetUrl === 'function') {
                return h('div', { style: box({ background: x.c.bg || 'transparent', 'border-radius': own(x.c.ownRadius, 0) + 'px' }) },
                    h('img', { src: x.assetUrl(id), alt: '', draggable: 'false', style: { width: '100%', height: '100%', 'object-fit': 'none', 'object-position': 'center', 'image-rendering': 'pixelated', 'pointer-events': 'none' } }));
            }
            return h('div', { class: 'cc-wv-placeholder', style: box(center({ color: x.c.tk.text_muted, border: '1px dashed ' + x.c.tk.text_muted, 'font-size': '18px' })) }, symbolGlyph('image'));
        },
        panel(x) {
            return h('div', { style: box({ background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 0), 'border-radius': x.c.radius + 'px' }) });
        },
        line(x) {
            const t = Math.max(1, Number(x.pv('thickness')) || 1);
            const vertical = x.pv('orientation') === 'vertical';
            const color = (x.w.style && x.w.style.accent_color) || x.c.tk.border;
            return h('div', { style: box({ overflow: 'visible', background: x.c.bg || 'transparent' }) }, h('div', { style: vertical
                ? { position: 'absolute', top: '0', bottom: '0', left: 'calc(50% - ' + t / 2 + 'px)', width: t + 'px', background: color, 'border-radius': t / 2 + 'px' }
                : { position: 'absolute', left: '0', right: '0', top: 'calc(50% - ' + t / 2 + 'px)', height: t + 'px', background: color, 'border-radius': t / 2 + 'px' } }));
        },
        switch(x) {
            // Track surface_alt (accent when checked); the knob is ui__st_knob: text fill, 2 px accent ring.
            const on = !!x.pv('checked');
            const inset = LV.SWITCH_INSET;
            const d = Math.max(4, x.w.h - 2 * inset);
            return h('div', { style: box({ background: on ? x.c.accent : fill(x.c, x.c.tk.surface_alt), border: border(x.c, 0), 'border-radius': own(x.c.ownRadius, Math.min(x.w.w, x.w.h) / 2) + 'px' }) },
                h('div', { style: { position: 'absolute', top: inset + 'px', left: on ? 'calc(100% - ' + (d + inset) + 'px)' : inset + 'px', width: d + 'px', height: d + 'px', 'box-sizing': 'border-box', 'border-radius': '50%', background: x.c.tk.text, border: '2px solid ' + x.c.tk.accent } }));
        },
        checkbox(x) {
            // LVGL marker: line height + 2 × pad, at the top-left; ui__st_input box, accent when checked, white tick.
            const on = !!x.pv('checked');
            const s = Math.round(parseInt(x.font['font-size'], 10) * 1.2) + 2 * LV.MARKER_PAD;
            return h('div', { style: box(Object.assign({}, x.font, { display: 'flex', 'align-items': 'flex-start', gap: LV.PAD_SMALL + 'px', color: x.c.text, background: x.c.bg || 'transparent', border: border(x.c, 0), 'border-radius': own(x.c.ownRadius, 0) + 'px' })) },
                h('span', { class: 'cc-glyph', style: center({ width: s + 'px', height: s + 'px', flex: 'none', 'box-sizing': 'border-box', 'border-radius': Math.min(s / 2, x.c.tk.radius || 0) + 'px', border: Math.max(1, x.c.tk.border_width || 1) + 'px solid ' + x.c.tk.border, background: on ? x.c.accent : x.c.tk.surface, color: LV_WHITE }) }, on ? '✓' : ''),
                h('span', { style: { 'white-space': 'nowrap', overflow: 'hidden', 'text-overflow': 'clip', 'line-height': s + 'px' } }, String(x.pv('text') || '')));
        },
        slider(x) {
            // The track is the whole widget (vertical when taller than wide); the knob is the cross
            // size plus 2 × knob pad, centred on the indicator end (it overhangs the widget box).
            const r = ratio(Number(x.pv('value')), Number(x.pv('min')), Number(x.pv('max')));
            const vertical = x.w.h > x.w.w;
            const cross = vertical ? x.w.w : x.w.h;
            const k = cross + 2 * LV.KNOB_PAD;
            const pct = Math.round(r * 100);
            const radius = own(x.c.ownRadius, cross / 2) + 'px';
            const knob = { position: 'absolute', width: k + 'px', height: k + 'px', 'box-sizing': 'border-box', 'border-radius': '50%', background: x.c.tk.text, border: '2px solid ' + x.c.tk.accent };
            if (vertical) Object.assign(knob, { left: -LV.KNOB_PAD + 'px', top: 'calc(' + (100 - pct) + '% - ' + k / 2 + 'px)' });
            else Object.assign(knob, { top: -LV.KNOB_PAD + 'px', left: 'calc(' + pct + '% - ' + k / 2 + 'px)' });
            return h('div', { style: box({ overflow: 'visible' }) },
                h('div', { style: { position: 'absolute', left: '0', top: '0', right: '0', bottom: '0', overflow: 'hidden', 'box-sizing': 'border-box', 'border-radius': radius, background: fill(x.c, x.c.tk.surface_alt), border: border(x.c, 0) } },
                    h('div', { style: vertical
                        ? { position: 'absolute', left: '0', right: '0', bottom: '0', height: pct + '%', 'border-radius': radius, background: x.c.accent }
                        : { width: pct + '%', height: '100%', 'border-radius': radius, background: x.c.accent } })),
                h('div', { style: knob }));
        },
        arc(x) {
            // lv_arc: a square of min(w, h) at the top-left, outer edge on the box, 270° from 135°.
            const r = ratio(Number(x.pv('value')), Number(x.pv('min')), Number(x.pv('max')));
            const size = Math.min(x.w.w, x.w.h);
            const sw = Math.max(1, Math.min(LV.ARC_W, Math.floor(size / 2)));
            const rad = Math.max(0.5, size / 2 - sw / 2);
            const end = 135 + 270 * r;
            const knob = [size / 2 + rad * Math.cos(end * Math.PI / 180), size / 2 + rad * Math.sin(end * Math.PI / 180)];
            return h('div', { style: box({ overflow: 'visible', background: x.c.bg || 'transparent' }) },
                h('svg', { width: String(size), height: String(size), viewBox: '0 0 ' + size + ' ' + size, style: { position: 'absolute', left: '0', top: '0', overflow: 'visible' } },
                    h('path', { d: arcPath(size / 2, size / 2, rad, 135, 405), fill: 'none', stroke: x.c.tk.surface_alt, 'stroke-width': String(sw), 'stroke-linecap': 'round' }),
                    r > 0 ? h('path', { d: arcPath(size / 2, size / 2, rad, 135, end), fill: 'none', stroke: x.c.accent, 'stroke-width': String(sw), 'stroke-linecap': 'round' }) : null,
                    h('circle', { cx: String(Math.round(knob[0])), cy: String(Math.round(knob[1])), r: String(sw / 2 + LV.KNOB_PAD - 1), fill: x.c.tk.text, stroke: x.c.tk.accent, 'stroke-width': '2' })));
        },
        dropdown(x) {
            const opts = list(x.pv('options'));
            const sel = opts[Math.max(0, Math.min(opts.length - 1, Number(x.pv('selected')) || 0))] || '';
            return h('div', { style: box(Object.assign({}, x.font, { display: 'flex', 'align-items': 'center', 'justify-content': 'space-between', padding: '0 8px', color: x.c.text, background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 1), 'border-radius': x.c.radius + 'px', 'white-space': 'nowrap' })) },
                h('span', { style: { overflow: 'hidden', 'text-overflow': 'ellipsis' } }, sel), h('span', { class: 'cc-glyph' }, '▾'));
        },
        roller(x) {
            const opts = list(x.pv('options'));
            const rows = Math.max(1, Number(x.pv('visible_rows')) || 3);
            const sel = Math.max(0, Math.min(opts.length - 1, Number(x.pv('selected')) || 0));
            const half = Math.floor(rows / 2);
            const lines = [];
            for (let i = -half; i < rows - half; i++) lines.push({ text: opts[sel + i] || '', active: i === 0 });
            return h('div', { style: box(Object.assign({}, x.font, { display: 'flex', 'flex-direction': 'column', color: x.c.text, background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 1), 'border-radius': x.c.radius + 'px' })) },
                lines.map(l => h('div', { style: center({ flex: '1', 'min-height': '0', 'white-space': 'nowrap', overflow: 'hidden', color: l.active ? x.c.tk.on_accent : x.c.text, background: l.active ? x.c.accent : 'transparent' }) }, l.text)));
        },
        textarea(x) {
            // pad_small padding; password mode shows the generator's "*" bullet; LVGL draws the cursor only while focused.
            const text = String(x.pv('text') || '');
            const shown = text ? (x.pv('password') ? '*'.repeat(Math.min(text.length, 64)) : text) : String(x.pv('placeholder') || '');
            return h('div', { style: box(Object.assign({}, x.font, { padding: LV.PAD_SMALL + 'px', color: text ? x.c.text : x.c.tk.text_muted, background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 1), 'border-radius': x.c.radius + 'px', 'white-space': x.pv('one_line') ? 'nowrap' : 'pre-wrap', 'overflow-wrap': 'anywhere' })) }, shown);
        },
        spinbox(x) {
            // Generator: − and + are theme buttons (h × h) at both ends, 4 px from the value field;
            // lv_spinbox shows a sign only when the range includes negative values.
            const digits = Math.max(1, Number(x.pv('digits')) || 1);
            const decimals = Math.max(0, Math.min(digits - 1, Number(x.pv('decimals')) || 0));
            const v = Math.round(Number(x.pv('value')) || 0);
            const raw = String(Math.abs(v)).padStart(digits, '0');
            const sign = Number(x.pv('min')) < 0 ? (v < 0 ? '-' : '+') : '';
            const text = sign + (decimals ? raw.slice(0, raw.length - decimals) + '.' + raw.slice(raw.length - decimals) : raw);
            const b = buttonFace(x.theme, colors(x.theme, null));
            const btn = glyph => h('span', { class: 'cc-glyph', style: center(Object.assign({}, b.face, { width: x.w.h + 'px', height: '100%', flex: 'none', 'box-sizing': 'border-box' })) }, glyph);
            return h('div', { style: box(Object.assign({}, x.font, { display: 'flex', gap: '4px', color: x.c.text })) },
                btn(SYMBOLS.minus), h('div', { style: center({ flex: '1', 'min-width': '0', overflow: 'hidden', background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 1), 'border-radius': x.c.radius + 'px', 'box-sizing': 'border-box' }) }, text), btn(SYMBOLS.plus));
        },
        keypad(x) {
            // Built-in maps and keypadMap's custom rule: "\n" breaks rows, otherwise more than four
            // keys make rows of four; empty keys become a space. Card pads, item style ui__st_item.
            const layout = x.pv('layout');
            let rows;
            if (layout === 'custom') {
                const keys = list(x.pv('keys'));
                const breaks = keys.includes('\n');
                rows = [[]];
                keys.forEach((k, i) => {
                    if (k === '\n') { rows.push([]); return; }
                    if (!breaks && keys.length > 4 && i > 0 && i % 4 === 0) rows.push([]);
                    rows[rows.length - 1].push(k === '' ? ' ' : k);
                });
                rows = rows.filter(r => r.length);
            } else {
                rows = [['1', '2', '3'], ['4', '5', '6'], ['7', '8', '9'], layout === 'numeric_ok' ? ['⌫', '0', 'OK'] : ['.', '0', '⌫']];
            }
            return h('div', { style: box(Object.assign({}, x.font, { display: 'flex', 'flex-direction': 'column', gap: LV.PAD_SMALL + 'px', padding: LV.PAD_DEF + 'px', background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 1), 'border-radius': x.c.radius + 'px' })) },
                rows.map(r => h('div', { style: { display: 'flex', gap: LV.PAD_SMALL + 'px', flex: '1', 'min-height': '0' } },
                    r.map(k => h('span', { style: center({ flex: '1', 'min-width': '0', overflow: 'hidden', 'white-space': 'pre', background: x.c.tk.surface_alt, color: x.c.tk.text, 'border-radius': (x.c.tk.radius || 0) + 'px' }) }, k)))));
        },
        bar(x) {
            // lv_bar: round track (vertical when taller than wide, filling from the bottom).
            const r = ratio(Number(x.pv('value')), Number(x.pv('min')), Number(x.pv('max')));
            const vertical = x.w.h > x.w.w;
            const pct = Math.round(r * 100);
            return h('div', { style: box({ background: fill(x.c, x.c.tk.surface_alt), border: border(x.c, 0), 'border-radius': own(x.c.ownRadius, Math.min(x.w.w, x.w.h) / 2) + 'px' }) },
                h('div', { style: vertical
                    ? { position: 'absolute', left: '0', right: '0', bottom: '0', height: pct + '%', background: x.c.accent, 'border-radius': 'inherit' }
                    : { width: pct + '%', height: '100%', background: x.c.accent, 'border-radius': 'inherit' } }));
        },
        spinner(x) {
            // lv_spinner: an arc square at the top-left, full track, 200° indicator (one frame).
            const size = Math.min(x.w.w, x.w.h);
            const sw = Math.max(1, Math.min(LV.ARC_W, Math.floor(size / 2)));
            const rad = Math.max(0.5, size / 2 - sw / 2);
            return h('div', { style: box({ overflow: 'visible', background: x.c.bg || 'transparent' }) },
                h('svg', { width: String(size), height: String(size), viewBox: '0 0 ' + size + ' ' + size, style: { position: 'absolute', left: '0', top: '0' } },
                    h('circle', { cx: String(size / 2), cy: String(size / 2), r: String(rad), fill: 'none', stroke: x.c.tk.surface_alt, 'stroke-width': String(sw) }),
                    h('path', { d: arcPath(size / 2, size / 2, rad, -90, 110), fill: 'none', stroke: x.c.accent, 'stroke-width': String(sw), 'stroke-linecap': 'round' })));
        },
        led(x) {
            // lv_led mixes its colour with black by brightness (80..255, off = 80); the glow grows with it.
            const color = x.pv('color') || x.c.accent;
            const bright = x.pv('on') ? Math.max(LED_MIN, Math.min(255, Number(x.pv('brightness')) || 0)) : LED_MIN;
            const k = (bright - LED_MIN) / (255 - LED_MIN);
            const lit = shade(color, bright);
            return h('div', { style: box({ overflow: 'visible', 'border-radius': Math.min(x.w.w, x.w.h) / 2 + 'px', background: lit, 'box-shadow': k > 0 ? '0 0 ' + Math.round(12 * k) + 'px ' + Math.round(4 * k) + 'px ' + lit : 'none' }) });
        },
        chart(x) {
            // Panel style, content inset by pad_small; lv_chart's 3 + 5 division lines; sample data
            // (the device starts empty); line series with point dots, bars per lv_chart's gaps.
            const W = x.w.w, H = x.w.h;
            const bw = x.c.bw === null ? (x.c.tk.border_width || 0) : x.c.bw;
            const cw = Math.max(1, W - 2 * (bw + LV.PAD_SMALL)), ch = Math.max(1, H - 2 * (bw + LV.PAD_SMALL));
            const points = Math.max(2, Number(x.pv('points')) || 2);
            const min = Number(x.pv('min')), max = Number(x.pv('max'));
            const series = list(x.pv('series'));
            const hues = [x.c.accent, x.c.tk.success, x.c.tk.warning, x.c.tk.danger];
            const val = (s, i) => min + (max - min) * (0.5 + 0.35 * Math.sin(i * 0.9 + s * 1.7));
            const y = v => Math.round(ch - ratio(v, min, max) * ch);
            const grid = [0, 1, 2].map(k => h('line', { x1: '0', x2: String(cw), y1: String(Math.round(ch * k / 2)), y2: String(Math.round(ch * k / 2)), stroke: x.c.tk.border, 'stroke-width': '1' }))
                .concat([0, 1, 2, 3, 4].map(k => h('line', { y1: '0', y2: String(ch), x1: String(Math.round(cw * k / 4)), x2: String(Math.round(cw * k / 4)), stroke: x.c.tk.border, 'stroke-width': '1' })));
            const marks = series.map((name, s) => {
                const hue = hues[s % hues.length];
                if (x.pv('kind') === 'bar') {
                    const block = Math.max(1, Math.floor((cw - (points - 1) * LV.PAD_SMALL) / points));
                    const col = Math.max(1, Math.floor((block - (series.length - 1) * LV.SER_GAP) / series.length));
                    return h('g', {}, Array.from({ length: points }, (_, i) => h('rect', { x: String(i * (block + LV.PAD_SMALL) + s * (col + LV.SER_GAP)), y: String(y(val(s, i))), width: String(col), height: String(Math.max(0, ch - y(val(s, i)))), fill: hue })));
                }
                const px = i => Math.round(i * cw / (points - 1));
                const pts = Array.from({ length: points }, (_, i) => px(i) + ',' + y(val(s, i))).join(' ');
                return h('g', {}, h('polyline', { points: pts, fill: 'none', stroke: hue, 'stroke-width': '2' }),
                    Array.from({ length: points }, (_, i) => h('circle', { cx: String(px(i)), cy: String(y(val(s, i))), r: String(LV.POINT_R), fill: hue })));
            });
            return h('div', { style: box({ background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 0), 'border-radius': x.c.radius + 'px' }) },
                h('svg', { width: String(cw), height: String(ch), viewBox: '0 0 ' + cw + ' ' + ch, style: { position: 'absolute', left: LV.PAD_SMALL + 'px', top: LV.PAD_SMALL + 'px', overflow: 'visible' } }, grid, marks));
        },
        gauge(x) {
            // lv_scale ROUND_INNER on a centred square: panel fill and border, a 2 px arc and equal
            // 5 px ticks in the text colour, labels in the theme font, accent needle of gaugeNeedle().
            const size = Math.min(x.w.w, x.w.h);
            const min = Number(x.pv('min')), max = Number(x.pv('max'));
            const ticks = Math.max(2, Number(x.pv('ticks')) || 2);
            const every = Math.max(1, Number(x.pv('label_every')) || 1);
            const bw = x.c.bw === null ? (x.c.tk.border_width || 0) : x.c.bw;
            const c = size / 2, rad = Math.max(1, size / 2 - bw);
            const fs = parseInt(x.font['font-size'], 10);
            const at = (a, r) => [c + r * Math.cos(a * Math.PI / 180), c + r * Math.sin(a * Math.PI / 180)].map(v => String(Math.round(v * 10) / 10));
            const parts = [
                h('circle', { cx: String(c), cy: String(c), r: String(Math.max(0, size / 2 - bw / 2)), fill: fill(x.c, x.c.tk.surface), stroke: bw > 0 ? x.c.border : 'none', 'stroke-width': String(bw) }),
                h('path', { d: arcPath(c, c, Math.max(0.5, rad - 1), 135, 405), fill: 'none', stroke: x.c.text, 'stroke-width': '2' })
            ];
            for (let i = 0; i < ticks; i++) {
                const a = 135 + 270 * i / (ticks - 1);
                const p1 = at(a, rad), p2 = at(a, Math.max(0, rad - LV.TICK));
                parts.push(h('line', { x1: p1[0], y1: p1[1], x2: p2[0], y2: p2[1], stroke: x.c.text, 'stroke-width': '2' }));
                if (i % every === 0) {
                    const p = at(a, Math.max(0, rad - LV.TICK - 4 - fs * 0.6));
                    parts.push(h('text', { x: p[0], y: p[1], fill: x.c.text, 'font-family': x.font['font-family'], 'font-size': String(fs), 'text-anchor': 'middle', 'dominant-baseline': 'middle' }, String(Math.round(min + (max - min) * i / (ticks - 1)))));
                }
            }
            const angle = 135 + 270 * ratio(Number(x.pv('value')), min, max);
            const tip = at(angle, Math.max(Math.floor(size / 2) - 16, 4));
            parts.push(h('line', { x1: String(c), y1: String(c), x2: tip[0], y2: tip[1], stroke: x.c.accent, 'stroke-width': '3', 'stroke-linecap': 'round' }));
            return h('div', { style: box(center({ overflow: 'visible' })) }, h('svg', { width: String(size), height: String(size), viewBox: '0 0 ' + size + ' ' + size, style: { flex: 'none', overflow: 'visible' } }, parts));
        },
        table(x) {
            // ui__style_items: input box; cells are ui__st_item (surface_alt, rounded) with LVGL's
            // pad_normal padding and 1 px top/bottom cell borders; cell text wraps.
            const cols = Math.max(1, Number(x.pv('columns')) || 1);
            const rows = Math.max(1, Number(x.pv('rows')) || 1);
            const cells = list(x.pv('cells'));
            const grid = [];
            for (let r = 0; r < rows; r++) for (let col = 0; col < cols; col++) {
                grid.push(h('div', { style: { padding: LV.PAD_DEF + 'px', 'min-height': '1.2em', overflow: 'hidden', 'white-space': 'pre-wrap', 'overflow-wrap': 'anywhere', background: x.c.tk.surface_alt, color: x.c.tk.text, 'border-top': '1px solid ' + x.c.tk.border, 'border-bottom': '1px solid ' + x.c.tk.border, 'border-radius': (x.c.tk.radius || 0) + 'px' } }, cells[r * cols + col] || ''));
            }
            return h('div', { style: box(Object.assign({}, x.font, { display: 'grid', 'grid-template-columns': 'repeat(' + cols + ', minmax(0, 1fr))', 'grid-auto-rows': 'min-content', 'align-content': 'start', color: x.c.text, background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 1), 'border-radius': x.c.radius + 'px' })) }, grid);
        },
        qrcode(x) {
            // The generator draws black modules on white, on a centred min(w, h) square.
            const n = 21;
            const size = Math.min(x.w.w, x.w.h);
            const text = String(x.pv('text') || '');
            let seed = 2166136261;
            for (let i = 0; i < text.length; i++) seed = Math.imul(seed ^ text.charCodeAt(i), 16777619) >>> 0;
            const finder = (r, c) => [[0, 0], [0, n - 7], [n - 7, 0]].some(([fr, fc]) => r >= fr && r < fr + 7 && c >= fc && c < fc + 7);
            const finderOn = (r, c) => { const fr = r < 7 ? r : r - (n - 7); const fc = c < 7 ? c : c - (n - 7); const d = Math.max(Math.abs(fr - 3), Math.abs(fc - 3)); return d !== 2; };
            const cells = [];
            for (let r = 0; r < n; r++) for (let c = 0; c < n; c++) {
                seed = (Math.imul(seed, 1103515245) + 12345) >>> 0;
                const on = finder(r, c) ? finderOn(r, c) : (seed >>> 16) % 2 === 0;
                if (on) cells.push('M' + c + ' ' + r + 'h1v1h-1z');
            }
            return h('div', { style: box(center({ background: x.c.bg || 'transparent' })) },
                h('svg', { width: String(size), height: String(size), viewBox: '0 0 ' + n + ' ' + n, 'shape-rendering': 'crispEdges', style: { flex: 'none', background: QR_LIGHT } }, h('path', { d: cells.join(''), fill: QR_DARK })));
        },
        tabview(x) {
            // Generator: bg, tab bar surface_alt. LVGL tab buttons: the checked one gets accent text,
            // 20 % accent fill and a 4 px bottom border (for every bar position); the others the theme text.
            const tabs = list(x.pv('tabs'));
            const pos = x.pv('bar_position') || 'top';
            const size = Number(x.pv('bar_size')) || 40;
            const vertical = pos === 'left' || pos === 'right';
            const barStyle = { position: 'absolute', display: 'flex', 'flex-direction': vertical ? 'column' : 'row', background: x.c.tk.surface_alt, color: x.c.tk.text, 'box-sizing': 'border-box' };
            if (pos === 'top') Object.assign(barStyle, { left: '0', right: '0', top: '0', height: size + 'px' });
            if (pos === 'bottom') Object.assign(barStyle, { left: '0', right: '0', bottom: '0', height: size + 'px' });
            if (pos === 'left') Object.assign(barStyle, { left: '0', top: '0', bottom: '0', width: size + 'px' });
            if (pos === 'right') Object.assign(barStyle, { right: '0', top: '0', bottom: '0', width: size + 'px' });
            const active = Math.max(0, Math.min(tabs.length - 1, x.activeTab || 0));
            return h('div', { style: box(Object.assign({}, x.font, { background: fill(x.c, x.c.tk.bg), border: border(x.c, 0), 'border-radius': own(x.c.ownRadius, 0) + 'px', color: x.c.text })) },
                h('div', { class: 'cc-tabbar', style: barStyle }, tabs.map((label, i) => h('span', {
                    'data-cc-tab': String(i),
                    style: center({ flex: '1', 'min-width': '0', 'min-height': '0', overflow: 'hidden', 'white-space': 'nowrap', padding: '0 4px', 'box-sizing': 'border-box', color: i === active ? x.c.tk.accent : x.c.tk.text, background: i === active ? rgba(x.c.tk.accent, 0.2) : 'transparent', 'border-bottom': i === active ? LV.TAB_BORDER + 'px solid ' + x.c.tk.accent : 'none', 'padding-top': i === active ? LV.TAB_BORDER + 'px' : '0' })
                }, label))));
        },
        list(x) {
            // lv_list: pad_def side inset, item rows with a bottom border each (LVGL list buttons).
            const items = list(x.pv('items'));
            const rowH = Math.round(parseInt(x.font['font-size'], 10) * 2.2);
            return h('div', { style: box(Object.assign({}, x.font, { padding: '0 ' + LV.PAD_DEF + 'px', color: x.c.text, background: fill(x.c, x.c.tk.surface), border: border(x.c, x.c.tk.border_width || 1), 'border-radius': x.c.radius + 'px' })) },
                items.map(item => h('div', { style: { height: rowH + 'px', 'box-sizing': 'border-box', display: 'flex', 'align-items': 'center', padding: '0 8px', 'white-space': 'nowrap', overflow: 'hidden', color: x.c.tk.text, 'border-bottom': '1px solid ' + x.c.tk.border } }, item)));
        },
        msgbox(x) {
            // lv_msgbox: header bar (surface_alt, generator) with the title, growing content,
            // footer bar with theme buttons spaced evenly ("[ OK ]" in bracket themes).
            const buttons = list(x.pv('buttons'));
            const title = String(x.pv('title') || '');
            const b = buttonFace(x.theme, colors(x.theme, null));
            const bar = { flex: 'none', height: LV.BAR_H + 'px', 'box-sizing': 'border-box', display: 'flex' };
            return h('div', { class: 'cc-wv-modal', style: box(Object.assign({}, x.font, { display: 'flex', 'flex-direction': 'column', color: x.c.text, background: fill(x.c, x.c.tk.surface), border: border(x.c, Math.max(1, x.c.tk.border_width || 1)), 'border-radius': x.c.radius + 'px' })) },
                title ? h('div', { style: Object.assign({}, bar, { 'align-items': 'center', padding: '0 ' + LV.PAD_SMALL + 'px', background: x.c.tk.surface_alt, 'white-space': 'nowrap', overflow: 'hidden' }) }, h('span', { style: { overflow: 'hidden', 'text-overflow': 'ellipsis' } }, title)) : null,
                h('div', { style: { flex: '1', 'min-height': '0', overflow: 'hidden', padding: LV.PAD_SMALL + 'px', 'white-space': 'pre-wrap', 'overflow-wrap': 'anywhere' } }, String(x.pv('text') || '')),
                buttons.length ? h('div', { style: Object.assign({}, bar, { 'align-items': 'stretch', 'justify-content': 'space-evenly', padding: LV.PAD_SMALL + 'px' }) },
                    buttons.map(t => h('span', { style: center(Object.assign({}, b.face, { padding: '0 ' + LV.PAD_DEF + 'px', 'white-space': 'nowrap', 'box-sizing': 'border-box' })) }, buttonLabel(b.kind, '', t)))) : null);
        },
        canvas(x) {
            return h('div', { class: 'cc-wv-placeholder', style: box(center({ background: fill(x.c, x.c.tk.surface_alt), border: '1px dashed ' + x.c.tk.text_muted, color: x.c.tk.text_muted, 'font-size': '18px' })) }, '▦');
        },
        statusbar(x) {
            // ui__st_statusbar: surface, bottom border only, no radius, the theme's small font. An empty
            // title shows the screen title; Wi-Fi/clock follow the project settings when given.
            const set = x.settings;
            const parts = [];
            if (x.pv('show_wifi') && (!set || set.wifi !== 'off')) parts.push(h('span', { class: 'cc-glyph' }, symbolGlyph('wifi')));
            if (x.pv('show_clock') && (!set || !!set.statusbar_clock)) parts.push(h('span', {}, '12:34'));
            const bw = x.c.bw === null ? (x.c.tk.border_width || 0) : x.c.bw;
            return h('div', { style: box(Object.assign({}, x.font, { display: 'flex', 'align-items': 'center', 'justify-content': 'space-between', gap: '8px', padding: '0 6px', color: x.c.text, background: fill(x.c, x.c.tk.surface), 'border-bottom': bw > 0 ? bw + 'px solid ' + x.c.border : 'none', 'border-radius': own(x.c.ownRadius, 0) + 'px', 'white-space': 'nowrap', 'font-size': smallSize((x.theme && x.theme.font_size) || 16) + 'px' })) },
                h('span', { style: { overflow: 'hidden', 'text-overflow': 'ellipsis' } }, String(x.pv('title') || x.screenTitle || '')), h('span', { style: { display: 'flex', gap: '6px' } }, parts));
        }
    };

    // tree returns the preview node of one widget (its own visuals; the canvas adds children).
    // opts: assetUrl, activeTab, and for statusbars screenTitle and settings ({wifi, statusbar_clock}).
    function tree(spec, widget, theme, opts) {
        const o = opts || {};
        const renderer = spec && R[spec.type];
        const c = colors(theme, widget.style);
        const ctx = { w: widget, spec, theme: theme || {}, c, pv: name => P.model.propValue(widget, spec, name), font: fontStyle(theme, widget, spec), assetUrl: o.assetUrl, activeTab: o.activeTab || 0, screenTitle: o.screenTitle || '', settings: o.settings || null };
        const node = renderer ? renderer(ctx) : h('div', { class: 'cc-wv-placeholder', style: box(center({ border: '1px dashed ' + (c.tk.danger || 'currentColor'), color: c.tk.danger })) }, '?');
        node.attrs.class = ('cc-wv cc-wv-' + (spec ? spec.type : 'unknown') + ' ' + (node.attrs.class || '')).trim();
        return node;
    }

    // widget is the contract entry (overview): spec, widget, theme, assetUrl → HTMLElement.
    function widget(spec, w, theme, assetUrl, opts) {
        return dom(tree(spec, w, theme, Object.assign({}, opts, { assetUrl })));
    }

    // wifiSetup previews the template's Wi-Fi setup screen (a locked system screen) in the theme.
    function wifiSetup(theme, size, t) {
        const tk = (theme && theme.tokens) || {};
        const font = { 'font-family': fontFamily(theme && theme.font), 'font-size': ((theme && theme.font_size) || 16) + 'px', 'line-height': '1.25' };
        const qrSize = Math.max(64, Math.min(120, Math.round(Math.min(size.w, size.h) * 0.4)));
        const qr = tree({ type: 'qrcode', props: [{ name: 'text', kind: 'string', default: 'WIFI:T:WPA;S:CYD-Setup;P:12345678;;' }] }, { id: 'sys_qr', type: 'qrcode', x: 0, y: 0, w: qrSize, h: qrSize }, theme, {});
        return h('div', { class: 'cc-system-screen', style: box(Object.assign({}, font, { display: 'flex', 'flex-direction': 'column', gap: '8px', padding: '12px', color: tk.text, background: tk.bg })) },
            h('div', { style: { 'font-weight': '600', 'font-size': '1.1em' } }, t('cyd_creator.designer.system_wifi_title')),
            h('div', { style: { display: 'flex', gap: '12px', 'align-items': 'center', flex: '1', 'min-height': '0' } },
                h('div', { style: { position: 'relative', width: qrSize + 'px', height: qrSize + 'px', flex: 'none' } }, qr),
                h('div', { style: { display: 'flex', 'flex-direction': 'column', gap: '6px', 'min-width': '0' } },
                    h('div', {}, t('cyd_creator.designer.system_wifi_join')),
                    h('div', { style: { color: tk.text_muted } }, 'CYD-Setup-A1B2'),
                    h('div', { style: { color: tk.text_muted } }, t('cyd_creator.designer.system_wifi_password') + ' ••••••••'))),
            h('div', { style: center({ height: '36px', background: tk.accent, color: tk.on_accent, 'border-radius': (tk.radius || 0) + 'px' }) }, t('cyd_creator.designer.system_wifi_on_device')));
    }

    // Chrome icons of the designer panels (24px stroke paths, currentColor).
    const ICONS = {
        'zoom-out': 'M11 4a7 7 0 1 0 0 14a7 7 0 1 0 0-14zM20 20l-4-4M8 11h6',
        'zoom-in': 'M11 4a7 7 0 1 0 0 14a7 7 0 1 0 0-14zM20 20l-4-4M8 11h6M11 8v6',
        fit: 'M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5',
        grid: 'M4 4h16v16H4zM4 9.3h16M4 14.6h16M9.3 4v16M14.6 4v16',
        'align-left': 'M4 3v18M8 7h10v4H8zM8 14h6v4H8z',
        'align-hcenter': 'M12 3v18M6 7h12v4H6zM8 14h8v4H8z',
        'align-right': 'M20 3v18M6 7h10v4H6zM10 14h6v4h-6z',
        'align-top': 'M3 4h18M7 8h4v10H7zM14 8h4v6h-4z',
        'align-vcenter': 'M3 12h18M7 6h4v12H7zM14 8h4v8h-4z',
        'align-bottom': 'M3 20h18M7 6h4v10H7zM14 10h4v6h-4z',
        'distribute-h': 'M4 4v16M20 4v16M9 8h6v8H9z',
        'distribute-v': 'M4 4h16M4 20h16M8 9h8v6H8z',
        'z-front': 'M9 9h11v11H9zM4 15V4h11',
        'z-back': 'M4 4h11v11H4zM9 18v2h11V9h-2',
        'z-forward': 'M12 19V5M6 11l6-6 6 6',
        'z-backward': 'M12 5v14M6 13l6 6 6-6',
        lock: 'M6 11h12v9H6zM8 11V8a4 4 0 0 1 8 0v3',
        unlock: 'M6 11h12v9H6zM8 11V8a4 4 0 0 1 7.5-2',
        eye: 'M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12zM12 9a3 3 0 1 0 0 6a3 3 0 1 0 0-6z',
        'eye-off': 'M3 3l18 18M10.6 6.1A10 10 0 0 1 12 6c6 0 10 6 10 6a17 17 0 0 1-3 3.6M6.6 7.6C3.9 9.4 2 12 2 12s4 7 10 7c1.7 0 3.2-.5 4.5-1.2',
        trash: 'M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13',
        plus: 'M12 5v14M5 12h14',
        edit: 'M4 20h4L18 10l-4-4L4 16zM14 6l4 4',
        copy: 'M8 8h12v12H8zM4 16V4h12',
        up: 'M12 19V5M6 11l6-6 6 6',
        down: 'M12 5v14M6 13l6 6 6-6',
        star: 'M12 3l2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1-4.4-4.3 6.1-.9z',
        upload: 'M12 16V4M7 9l5-5 5 5M4 16v4h16v-4',
        code: 'M8 7l-5 5 5 5M16 7l5 5-5 5',
        close: 'M6 6l12 12M18 6L6 18'
    };
    function icon(name, cls) {
        return h('svg', { class: 'cc-icon' + (cls ? ' ' + cls : ''), viewBox: '0 0 24 24', width: '16', height: '16', fill: 'none', stroke: 'currentColor', 'stroke-width': '1.8', 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'aria-hidden': 'true', focusable: 'false' }, h('path', { d: ICONS[name] || ICONS.close }));
    }

    P.render = { h, dom, html, swap, esc, rgba, colors, fontFamily, fontStyle, symbolGlyph, tree, widget, wifiSetup, icon, ICONS, TYPES: Object.keys(R), FAMILIES, SYMBOLS, FONT_SIZES };
})();
