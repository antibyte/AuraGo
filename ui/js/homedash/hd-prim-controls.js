// HomeDash control primitives (static builders): toggle, rocker, pushButton, slider, knob, stepper, segmented, colourWheel and
// ctStrip. Each builder returns {el, set(state), destroy()} and writes only attributes, text content and custom properties;
// all painting is CSS (markup contract C2). No pointer or keyboard handling in M0.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.geometry || !E.icons || !E.colour || !E.prim || !E.prim.util || !E.prim.numeral || !E.prim.segReadout) {
        throw new Error('hd-prim-controls.js: load hd-core, hd-format, hd-colour, hd-icons, hd-geometry and hd-prim-displays first');
    }
    const { setAttr, setVars, el, clamp } = E.core, svgEl = E.core.svg, G = E.geometry, F = E.format, util = E.prim.util;
    const { applyState, writeValue, root, finish, deg, dash } = util;
    const langOf = o => (o && o.lang) || 'en';
    const finite = v => typeof v === 'number' && Number.isFinite(v);
    const sized = o => ({ size: o && o.size });
    const withVariant = (o, allowed, fallback) => Object.assign(sized(o), { variant: allowed.includes(o && o.variant) ? o.variant : fallback });
    const iconNode = name => (name ? E.icons.icon(name) : null);

    function toggle(opts) {
        const o = opts || {}, node = root('button', 'hd-toggle', sized(o), { type: 'button', role: 'switch', 'aria-label': o.label, 'aria-checked': 'false' });
        const track = el('span', { class: 'hd-toggle-track', 'aria-hidden': 'true' }, [el('span', { class: 'hd-toggle-glyph', 'data-glyph': 'on' }),
            el('span', { class: 'hd-toggle-glyph', 'data-glyph': 'off' }), el('span', { class: 'hd-toggle-thumb' })]);
        node.appendChild(track);
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true });
            setAttr(node, 'aria-checked', s.on ? 'true' : 'false'); setAttr(node, 'data-hd-on', !!s.on);
        }
        return finish(node, set);
    }

    function rocker(opts) {
        const o = opts || {}, node = root('button', 'hd-rocker', sized(o), { type: 'button', role: 'switch', 'aria-label': o.label, 'aria-checked': 'false' });
        const half = (which, legend) => el('span', { class: 'hd-rocker-half', 'data-half': which }, el('span', { class: 'hd-rocker-legend', text: legend }));
        const paddle = el('span', { class: 'hd-rocker-paddle' }, [half('on', 'I'), half('off', 'O')]);
        node.appendChild(el('span', { class: 'hd-rocker-bezel', 'aria-hidden': 'true' }, [paddle, el('span', { class: 'hd-rocker-led' })]));
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true });
            setAttr(node, 'aria-checked', s.on ? 'true' : 'false'); setAttr(node, 'data-hd-on', !!s.on);
        }
        return finish(node, set);
    }

    function pushButton(opts) {
        const o = opts || {}, dir = o.dir === -1 || o.dir === 1 ? o.dir : null;
        const node = root('button', 'hd-push', withVariant(o, ['chip', 'tile'], 'chip'), { type: 'button', 'data-dir': dir });
        const icon = iconNode(o.icon), label = el('span', { class: 'hd-push-label', text: o.label || '' });
        if (icon) node.appendChild(icon);
        node.appendChild(label);
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true, readonlyAria: false });
            setAttr(node, 'data-hd-on', !!s.on); label.textContent = typeof s.text === 'string' ? s.text : (o.label || '');
        }
        return finish(node, set);
    }

    function slider(opts) {
        const o = opts || {}, lang = langOf(o), vertical = o.variant === 'v';
        const node = root('div', 'hd-slider', withVariant(o, ['h', 'v'], 'h'), { role: 'slider', tabindex: 0, 'aria-orientation': vertical ? 'vertical' : 'horizontal', 'aria-label': o.label });
        node.appendChild(el('span', { class: 'hd-slider-track', 'aria-hidden': 'true' }, [el('span', { class: 'hd-slider-fill' }), el('span', { class: 'hd-slider-thumb' })]));
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true });
            writeValue(node, s, { min: o.min, max: o.max, decimals: s.decimals ?? o.decimals, lang, unit: s.unit ?? o.unit });
        }
        return finish(node, set);
    }

    function knob(opts) {
        const o = opts || {}, lang = langOf(o), start = finite(o.start) ? o.start : -135, sweep = finite(o.sweep) ? o.sweep : 270;
        const quantize = o.quantize > 0 ? o.quantize : 0, tickCount = o.ticks === undefined ? 10 : Math.max(0, Math.floor(o.ticks)), segment = !!o.activeSegment;
        const readoutKind = o.readout === 'seg7' || o.readout === 'none' ? o.readout : 'numeral', full = G.arc(100, 100, 88, start, sweep);
        const node = root('div', 'hd-dial', withVariant(o, ['ring', 'knob'], 'ring'), { role: 'slider', tabindex: 0, 'aria-label': o.label, 'data-hd-quantize': quantize > 0 ? quantize : null });
        const svg = svgEl('svg', { class: 'hd-dial-svg', viewBox: '0 0 200 200', 'aria-hidden': 'true' });
        svg.appendChild(svgEl('path', { class: 'hd-dial-ticks', d: tickCount > 0 ? G.ticks(100, 100, 80, 88, start, sweep, tickCount) : '' }));
        svg.appendChild(svgEl('path', { class: 'hd-dial-track', pathLength: 1000, d: full }));
        const bufs = {};
        if (segment) for (const k of ['a', 'b']) { bufs[k] = svgEl('path', { class: 'hd-dial-active', 'data-buf': k }); svg.appendChild(bufs[k]); }
        const arcEl = svgEl('path', { class: 'hd-dial-arc', pathLength: 1000, d: full }); svg.appendChild(arcEl);
        svg.appendChild(svgEl('g', { class: 'hd-dial-handle' }, svgEl('circle', { class: 'hd-dial-knob', cx: 100, cy: 12, r: 11 })));
        if (segment) svg.appendChild(svgEl('g', { class: 'hd-dial-current' }, svgEl('path', { class: 'hd-dial-marker', d: 'M100 0L106 9H94Z' })));
        node.appendChild(svg);
        node.appendChild(el('span', { class: 'hd-dial-cap', 'aria-hidden': 'true' }, el('span', { class: 'hd-dial-pointer' })));
        const readoutHost = el('span', { class: 'hd-dial-readout' });
        const readout = readoutKind === 'numeral' ? E.prim.numeral({ chars: 4, unit: o.unit, decimals: o.decimals, lang, size: o.size })
            : readoutKind === 'seg7' ? E.prim.segReadout({ digits: 4, decimals: o.decimals, unit: o.unit, lang, size: o.size }) : null;
        if (readout) readoutHost.appendChild(readout.el);
        node.appendChild(readoutHost);
        let legendMin = null, legendMax = null;
        if (o.legends) {
            legendMin = el('span', { class: 'hd-dial-legend', 'data-end': 'min' }); legendMax = el('span', { class: 'hd-dial-legend', 'data-end': 'max' });
            node.appendChild(legendMin); node.appendChild(legendMax);
        }
        let visible = null, visibleD = null;
        function updateSegment(w, s) {
            if (!segment) return;
            if (!finite(s.current) || w.v === null) {
                setAttr(node, 'data-buf', null); setVars(node, { '--hd-angle-cur': null }); visible = null; visibleD = null; return;
            }
            const cf = G.fraction(s.current, w.min, w.max), lo = start + Math.min(cf, w.frac) * sweep, hi = start + Math.max(cf, w.frac) * sweep;
            setVars(node, { '--hd-angle-cur': deg(start + cf * sweep) });
            const d = G.arc(100, 100, 88, lo, hi - lo);
            if (visible !== null && d === visibleD) return;
            const next = visible === 'a' ? 'b' : 'a';
            bufs[next].setAttribute('d', d); visible = next; visibleD = d; setAttr(node, 'data-buf', next);
        }
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true });
            const unit = s.unit ?? o.unit, decimals = s.decimals ?? o.decimals, w = writeValue(node, s, { min: o.min, max: o.max, decimals, lang, unit });
            setVars(node, { '--hd-angle': deg(start + w.frac * sweep) }); dash(arcEl, w.frac, quantize);
            if (readout) readout.set({ value: w.v, decimals, unit });
            updateSegment(w, s);
            if (legendMin) { legendMin.textContent = F.number(w.min, { lang }); legendMax.textContent = F.number(w.max, { lang }); }
        }
        return finish(node, set);
    }

    function stepper(opts) {
        const o = opts || {}, lang = langOf(o);
        const node = root('div', 'hd-stepper', sized(o), { role: 'group', 'aria-label': o.label });
        const button = (dir, name, label) => el('button', { class: 'hd-step', type: 'button', 'data-dir': dir, 'aria-label': label }, iconNode(name));
        const dec = button(-1, 'minus', o.decLabel), inc = button(1, 'plus', o.incLabel);
        const num = E.prim.numeral({ chars: 4, unit: o.unit, decimals: o.decimals, lang, size: o.size });
        const spin = el('span', { class: 'hd-step-value', role: 'spinbutton', tabindex: 0, 'aria-label': o.label }, num.el);
        node.appendChild(dec); node.appendChild(spin); node.appendChild(inc);
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true, targets: [spin] }); applyState(node, s, { interactive: true, targets: [dec, inc], readonlyAria: false });
            const unit = s.unit ?? o.unit, decimals = s.decimals ?? o.decimals, w = writeValue(spin, s, { min: o.min, max: o.max, decimals, lang, unit });
            num.set({ value: w.v, decimals, unit });
            if (w.v !== null && w.v <= w.min) setAttr(dec, 'aria-disabled', true);
            if (w.v !== null && w.v >= w.max) setAttr(inc, 'aria-disabled', true);
        }
        return finish(node, set);
    }

    function segmented(opts) {
        const o = opts || {}, list = (Array.isArray(o.options) ? o.options : []).filter(x => x && typeof x.id === 'string');
        const node = root('div', 'hd-seg', sized(o), { role: 'radiogroup', 'aria-label': o.label });
        node.appendChild(el('span', { class: 'hd-seg-indicator', 'aria-hidden': 'true' }));
        const buttons = list.map(opt => { const icon = iconNode(opt.icon);
            const b = el('button', { class: 'hd-seg-opt', type: 'button', role: 'radio', 'data-hd-option': opt.id, 'aria-checked': 'false' }, [icon, el('span', { class: 'hd-seg-label', text: opt.label || '' })]);
            node.appendChild(b); return b; });
        function render(index) {
            setVars(node, { '--hd-index': index });
            buttons.forEach((b, i) => { setAttr(b, 'aria-checked', i === index ? 'true' : 'false'); setAttr(b, 'tabindex', i === (index >= 0 ? index : 0) ? 0 : -1); });
        }
        setVars(node, { '--hd-count': list.length }); render(-1);
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true });
            render(list.findIndex(x => x.id === s.value));
        }
        return finish(node, set);
    }

    // Paints the HSV disc once (hue clockwise from 12 o'clock, saturation = radius, value 1) with a one pixel soft edge.
    function paintWheel(canvas, px) {
        if (typeof canvas.getContext !== 'function') return;
        const ctx = canvas.getContext('2d'); if (!ctx || typeof ctx.createImageData !== 'function') return;
        const img = ctx.createImageData(px, px), c = px / 2, data = img.data;
        for (let y = 0; y < px; y++) for (let x = 0; x < px; x++) {
            const dx = (x + 0.5 - c) / c, dy = (y + 0.5 - c) / c, r = Math.sqrt(dx * dx + dy * dy), i = (y * px + x) * 4;
            if (r > 1) continue;
            const hue = (Math.atan2(dx, -dy) * 180 / Math.PI + 360) % 360, rgb = E.colour.hsvToRgb(hue, r, 1);
            data[i] = Math.round(rgb[0]); data[i + 1] = Math.round(rgb[1]); data[i + 2] = Math.round(rgb[2]); data[i + 3] = Math.round(clamp((1 - r) * c, 0, 1) * 255);
        }
        ctx.putImageData(img, 0, 0);
    }

    function colourWheel(opts) {
        const o = opts || {}, diameter = finite(o.diameter) && o.diameter > 0 ? o.diameter : 200;
        const dpr = finite(window.devicePixelRatio) && window.devicePixelRatio > 0 ? window.devicePixelRatio : 1, px = Math.max(1, Math.round(diameter * dpr));
        const node = root('div', 'hd-wheel', sized(o), { role: 'img', 'aria-label': o.label });
        const canvas = el('canvas', { class: 'hd-wheel-canvas', width: px, height: px, 'aria-hidden': 'true' });
        node.appendChild(canvas); node.appendChild(el('span', { class: 'hd-wheel-thumb', 'aria-hidden': 'true' }));
        paintWheel(canvas, px);
        function set(state) {
            const s = state || {}; applyState(node, s);
            const hue = finite(s.hue) ? ((s.hue % 360) + 360) % 360 : 0, sat = finite(s.sat) ? clamp(s.sat, 0, 1) : 0, rad = hue * Math.PI / 180;
            const rgb = E.colour.hsvToRgb(hue, sat, 1).map(n => clamp(Math.round(n), 0, 255));
            setVars(node, { '--hd-wheel-x': clamp(0.5 + 0.5 * sat * Math.sin(rad), 0, 1).toFixed(4), '--hd-wheel-y': clamp(0.5 - 0.5 * sat * Math.cos(rad), 0, 1).toFixed(4), '--hd-wheel-rgb': rgb.join(' ') });
            setAttr(node, 'aria-label', typeof s.valueText === 'string' && s.valueText ? s.valueText : o.label);
        }
        return finish(node, set);
    }

    function ctStrip(opts) {
        const o = opts || {}, lang = langOf(o), min = finite(o.min) ? o.min : 2200, max = finite(o.max) ? o.max : 6500;
        const node = root('div', 'hd-ct', sized(o), { role: 'slider', tabindex: 0, 'aria-label': o.label });
        node.appendChild(el('span', { class: 'hd-ct-track', 'aria-hidden': 'true' })); node.appendChild(el('span', { class: 'hd-ct-thumb', 'aria-hidden': 'true' }));
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true });
            writeValue(node, s, { min, max, decimals: 0, grouping: false, unit: 'K', lang });
        }
        return finish(node, set);
    }

    Object.assign(E.prim, { toggle, rocker, pushButton, slider, knob, stepper, segmented, colourWheel, ctStrip });
})();
