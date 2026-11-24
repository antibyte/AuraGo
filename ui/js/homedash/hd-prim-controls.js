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
    const { applyState, writeValue, root, finish, deg, dash, fmtN, leadArc } = util;
    const langOf = o => (o && o.lang) || 'en';
    const finite = v => typeof v === 'number' && Number.isFinite(v);
    const sized = o => ({ size: o && o.size });
    const withVariant = (o, allowed, fallback) => Object.assign(sized(o), { variant: allowed.includes(o && o.variant) ? o.variant : fallback });
    const iconNode = name => (name ? E.icons.icon(name) : null);

    function toggle(opts) {
        const o = opts || {}, node = root('button', 'hd-toggle', sized(o), { type: 'button', role: 'switch', 'aria-label': o.label, 'aria-checked': 'false' });
        const track = el('span', { class: 'hd-toggle-track', 'aria-hidden': 'true' }, [el('span', { class: 'hd-toggle-glyph', 'data-glyph': 'on' }),
            el('span', { class: 'hd-toggle-glyph', 'data-glyph': 'off' }), el('span', { class: 'hd-toggle-thumb' })]);
        let key = null;
        if (o.decor) { key = el('span', { class: 'hd-toggle-key', text: 'O' }); track.insertBefore(key, track.querySelector('.hd-toggle-thumb')); }
        node.appendChild(track);
        if (o.decor) {
            node.appendChild(el('span', { class: 'hd-toggle-legend', 'data-glyph': 'off', text: 'O' })); node.appendChild(el('span', { class: 'hd-toggle-legend', 'data-glyph': 'on', text: 'I' }));
            node.appendChild(el('span', { class: 'hd-toggle-led' }));
        }
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true });
            setAttr(node, 'aria-checked', s.on ? 'true' : 'false'); setAttr(node, 'data-hd-on', !!s.on);
            if (key) key.textContent = s.on ? 'I' : 'O';
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
        // Dots: o.dots.count circles of radius o.dots.size at radius o.dots.r, evenly over the sweep (first at the start, last at the end).
        const dotCfg = o.dots && finite(o.dots.count) && o.dots.count > 0 ? { count: Math.floor(o.dots.count), r: o.dots.r, size: o.dots.size, lit: !!o.dots.lit } : null;
        const dotShapes = [];
        if (dotCfg) for (let i = 0; i < dotCfg.count; i++) {
            const p = G.point(100, 100, dotCfg.r, start + (dotCfg.count > 1 ? i / (dotCfg.count - 1) : 0) * sweep), x0 = fmtN(p[0] - dotCfg.size), x1 = fmtN(p[0] + dotCfg.size), y = fmtN(p[1]), rr = fmtN(dotCfg.size);
            dotShapes.push('M' + x0 + ' ' + y + 'A' + rr + ' ' + rr + ' 0 1 1 ' + x1 + ' ' + y + 'A' + rr + ' ' + rr + ' 0 1 1 ' + x0 + ' ' + y + 'Z');
        }
        const dotsLit = dotCfg && dotCfg.lit ? svgEl('path', { class: 'hd-dial-dots-lit', d: '' }) : null;
        if (dotCfg) { svg.appendChild(svgEl('path', { class: 'hd-dial-dots', d: dotShapes.join('') })); if (dotsLit) svg.appendChild(dotsLit); }
        svg.appendChild(svgEl('path', { class: 'hd-dial-ticks', d: tickCount > 0 ? G.ticks(100, 100, 80, 88, start, sweep, tickCount) : '' }));
        const majors = o.ticksMajor > 0 ? Math.floor(o.ticksMajor) : 0;
        if (majors > 0) svg.appendChild(svgEl('path', { class: 'hd-dial-ticks-major', d: G.ticks(100, 100, 76, 88, start, sweep, majors) }));
        const numeralCount = Math.max(0, Math.min(6, Math.floor(finite(o.numerals) ? o.numerals : 0))), nMin = finite(o.min) ? o.min : 0, nMax = finite(o.max) ? o.max : 100;
        for (let i = 0; i < numeralCount; i++) {
            const t = i / Math.max(1, numeralCount - 1), p = G.point(100, 100, 67, start + sweep * t), v = nMin + t * (nMax - nMin);
            svg.appendChild(svgEl('text', { class: 'hd-dial-numeral', x: fmtN(p[0]), y: fmtN(p[1]), text: F.number(v, { lang, decimals: Number.isInteger(v) ? 0 : o.decimals }) }));
        }
        svg.appendChild(svgEl('path', { class: 'hd-dial-track', pathLength: 1000, d: full }));
        const bufs = {};
        if (segment) for (const k of ['a', 'b']) { bufs[k] = svgEl('path', { class: 'hd-dial-active', 'data-buf': k }); svg.appendChild(bufs[k]); }
        const glowEl = o.glow ? svgEl('path', { class: 'hd-dial-glow', pathLength: 1000, d: full }) : null; if (glowEl) svg.appendChild(glowEl);
        const arcEl = svgEl('path', { class: 'hd-dial-arc', pathLength: 1000, d: full }); svg.appendChild(arcEl);
        const leadEl = o.lead && quantize > 0 ? svgEl('path', { class: 'hd-dial-lead', d: '' }) : null; if (leadEl) svg.appendChild(leadEl);
        if (o.gaps > 0) svg.appendChild(svgEl('path', { class: 'hd-dial-gaps', pathLength: 1000, d: full }));
        svg.appendChild(svgEl('g', { class: 'hd-dial-handle' }, [o.handleShadow ? svgEl('circle', { class: 'hd-dial-knob-shadow', cx: 100, cy: 13.5, r: 11 }) : null,
            svgEl('circle', { class: 'hd-dial-knob', cx: 100, cy: 12, r: 11 })]));
        if (segment) svg.appendChild(svgEl('g', { class: 'hd-dial-current' }, [svgEl('path', { class: 'hd-dial-marker', d: 'M100 0L106 9H94Z' }),
            o.needle ? svgEl('path', { class: 'hd-dial-needle', d: 'M100 14V42' }) : null]));
        node.appendChild(svg);
        if (o.hardware) node.appendChild(el('span', { class: 'hd-dial-knurl', 'aria-hidden': 'true' }));
        node.appendChild(el('span', { class: 'hd-dial-cap', 'aria-hidden': 'true' }, el('span', { class: 'hd-dial-pointer' })));
        const readoutHost = el('span', { class: 'hd-dial-readout' });
        const readout = readoutKind === 'numeral' ? E.prim.numeral({ chars: 4, unit: o.unit, decimals: o.decimals, lang, size: o.size })
            : readoutKind === 'seg7' ? E.prim.segReadout({ digits: 4, decimals: o.decimals, unit: o.unit, lang, size: o.size }) : null;
        // Readout children: caption main, action, main readout, word, sub (icon + numeral or seg7), caption sub.
        const extra = !!o.extra, caps = Array.isArray(o.captions) ? o.captions : null;
        const caption = (which, text) => el('span', { class: 'hd-dial-caption', 'data-for': which, text: typeof text === 'string' ? text : '' });
        const actionEl = extra ? el('span', { class: 'hd-dial-action' }) : null, wordEl = o.word ? el('span', { class: 'hd-dial-word' }) : null;
        const subOpts = { chars: 4, unit: o.unit, decimals: o.decimals, lang, size: o.size }, subRead = !extra ? null
            : o.subReadout === 'seg7' ? E.prim.segReadout(Object.assign({ digits: 4 }, subOpts)) : E.prim.numeral(subOpts);
        if (extra && caps) readoutHost.appendChild(caption('main', caps[0]));
        if (actionEl) readoutHost.appendChild(actionEl);
        if (readout) readoutHost.appendChild(readout.el);
        if (wordEl) readoutHost.appendChild(wordEl);
        if (subRead) { readoutHost.appendChild(el('span', { class: 'hd-dial-sub' }, [iconNode(o.subIcon), subRead.el])); if (caps) readoutHost.appendChild(caption('sub', caps[1])); }
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
            if (glowEl) dash(glowEl, w.frac, quantize);
            if (leadEl) leadEl.setAttribute('d', leadArc(start, sweep, quantize, w.frac));
            if (dotsLit) dotsLit.setAttribute('d', dotShapes.slice(0, Math.round(w.frac * dotCfg.count)).join(''));
            if (readout) readout.set({ value: w.v, decimals, unit });
            if (wordEl) { const word = typeof s.word === 'string' ? s.word : ''; wordEl.textContent = word; setAttr(node, 'data-hd-word', word !== ''); }
            if (actionEl) actionEl.textContent = typeof s.action === 'string' ? s.action : '';
            if (subRead) subRead.set({ value: finite(s.sub) ? s.sub : null, decimals, unit });
            updateSegment(w, s);
            if (legendMin) { legendMin.textContent = F.number(w.min, { lang }); legendMax.textContent = F.number(w.max, { lang }); }
        }
        return finish(node, set);
    }

    function stepper(opts) {
        const o = opts || {}, lang = langOf(o);
        const node = root('div', 'hd-stepper', sized(o), { role: 'group', 'aria-label': o.label });
        const button = (dir, name, label) => el('button', { class: 'hd-step', type: 'button', 'data-dir': dir, 'aria-label': label }, iconNode(name));
        const dec = button(-1, 'minus', o.decLabel || String.fromCodePoint(0x2212)), inc = button(1, 'plus', o.incLabel || '+');
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
        if (o.rotary) node.appendChild(el('span', { class: 'hd-seg-rotary', 'aria-hidden': 'true' }, el('span', { class: 'hd-seg-rotary-pointer' })));
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
