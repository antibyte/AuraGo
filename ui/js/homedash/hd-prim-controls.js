// HomeDash control primitives: toggle, rocker, pushButton, slider, knob, stepper, segmented, colourWheel, ctStrip and picker. Each
// builder returns {el, set(state), destroy()} and writes only attributes, text content and custom properties; all painting is CSS
// (markup contract C2). Without `input` a builder is static and its markup is the M0 markup. With `opts.input` (SCR-M3-4) it binds
// pointer and keyboard through E.input (loaded lazily) and destroy() unbinds. Input callbacks by control: toggle/rocker/pushButton/picker
// {enabled, onBlocked, onActivate, onHold}; slider/ctStrip {range, onInput, onCommit, onKey, onToggle, onHold, enabled, onBlocked};
// knob adds onToggle (centre tap, Enter, Space); stepper {range, onStep(v)}; segmented {onPick(id, viaKeyboard)}; colourWheel
// {onInput, onCommit, onKey}({h 0-360, s 0-100}). A set() during a drag keeps the dragged value (input lock); a key press paints
// its value until the next set(). binder options (win, setTimeout, clearTimeout, now) pass through for tests.
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

    const inputApi = () => { if (!E.input) throw new Error('hd-prim-controls.js: load hd-input.js for interactive controls'); return E.input; };
    // The gate and the injectable clock every binder understands.
    const gate = input => ({ enabled: input.enabled, onBlocked: input.onBlocked, win: input.win, setTimeout: input.setTimeout, clearTimeout: input.clearTimeout, now: input.now });
    const open = (input, ev) => { if (!input.enabled || input.enabled()) return true; if (input.onBlocked) input.onBlocked(ev); return false; };
    const plain = ev => ev.defaultPrevented !== true && !ev.altKey && !ev.ctrlKey && !ev.metaKey;
    const stop = ev => { if (typeof ev.preventDefault === 'function') ev.preventDefault(); };
    const all = offs => () => offs.forEach(off => off());
    // destroy() runs the unbinders first, then the builder's own destroy (removes the node); both are idempotent.
    function bound(inst, off) { if (!off) return inst; const base = inst.destroy; inst.destroy = () => { off(); base(); }; return inst; }
    function bindPress(node, input) { setAttr(node, 'data-hd-input', 'press'); return inputApi().press(node, input); }
    // Input lock: state() is what is shown. show(patch, lock) paints a local value; while lock is true (a drag) set() keeps the patch.
    function lockPaint(paint) {
        let cur = {}, eff = {}, held = null, locked = false;
        const merged = () => Object.assign({}, cur, held, { valueText: null });
        return { set(state) { cur = state || {}; eff = locked && held ? merged() : cur; paint(eff); },
            show(patch, lock) { held = patch; locked = lock; eff = merged(); paint(eff); }, state: () => eff };
    }
    const numberOf = v => (finite(v) ? v : null);
    // Slider and CT strip: pointer on `measure()` (axis x or y), keys step the range. `show` paints, `current` reads the shown value.
    function bindLinear(node, measure, axis, input, show, current) {
        const I = inputApi(), r = () => input.range(), at = p => I.valueAt(I.fraction(p.x, p.y, measure(), axis), r());
        setAttr(node, 'data-hd-input', 'drag');
        const offG = I.gesture(node, Object.assign(gate(input), { onHold: input.onHold,
            onTap: p => { const v = at(p); show(v, false); input.onCommit(v); },
            onDrag: p => { const v = at(p); show(v, true); input.onInput(v); },
            onDragEnd: p => { const v = at(p); show(v, false); input.onCommit(v); } }));
        const offK = I.keys(node, Object.assign(gate(input), { onHold: input.onHold, onToggle: input.onToggle, range: r, get: current, onKey: v => { show(v, false); input.onKey(v); } }));
        return all([offG, offK]);
    }
    // Dial: a tap in the centre toggles, elsewhere it sets the value; a drag that starts in the centre is vertical, otherwise angular (15 deg dead zone).
    function bindDial(node, input, start, sweep, show, current) {
        const I = inputApi(), r = () => input.range(), rectOf = () => node.getBoundingClientRect();
        const frac = v => { const g = I.range(r()); return finite(v) ? clamp((v - g.min) / (g.max - g.min), 0, 1) : 0; };
        const centre = p => { const q = p.rect || rectOf(); return Math.hypot(p.x - (q.left + q.width / 2), p.y - (q.top + q.height / 2)) < 0.31 * q.width; };
        const angular = (p, last) => I.dialFraction(p.x, p.y, p.rect || rectOf(), last, start, sweep);
        let mode = 'angular', from = 0, last = 0, lastV = null;
        setAttr(node, 'data-hd-input', 'drag');
        const offG = I.gesture(node, Object.assign(gate(input), { rect: rectOf, onHold: input.onHold,
            onTap: p => { if (centre(p)) { if (input.onToggle) input.onToggle(); return; } const v = I.valueAt(angular(p, frac(current())), r()); show(v, false); input.onCommit(v); },
            onDragStart: p => { mode = centre(p) ? 'vertical' : 'angular'; from = last = frac(current()); lastV = null; },
            onDrag: (p, s0) => { last = mode === 'vertical' ? I.verticalFraction(from, p.y - s0.y) : angular(p, last); lastV = I.valueAt(last, r()); show(lastV, true); input.onInput(lastV); },
            onDragEnd: () => { if (lastV === null) return; const v = lastV; show(v, false); input.onCommit(v); } }));
        const offK = I.keys(node, Object.assign(gate(input), { onHold: input.onHold, onToggle: input.onToggle, range: r, get: current, onKey: v => { show(v, false); input.onKey(v); } }));
        return all([offG, offK]);
    }

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
        return bound(finish(node, set), o.input ? bindPress(node, o.input) : null);
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
        return bound(finish(node, set), o.input ? bindPress(node, o.input) : null);
    }

    function pushButton(opts) {
        const o = opts || {}, dir = o.dir === -1 || o.dir === 1 ? o.dir : null;
        const node = root('button', 'hd-push', withVariant(o, ['chip', 'tile'], 'chip'), { type: 'button', 'data-dir': dir, role: o.toggle ? 'switch' : null, 'aria-checked': o.toggle ? 'false' : null });
        let iconName = o.icon, icon = iconNode(iconName); const label = el('span', { class: 'hd-push-label', text: o.label || '' });
        if (icon) node.appendChild(icon);
        node.appendChild(label);
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true, readonlyAria: o.toggle ? undefined : false });
            setAttr(node, 'data-hd-on', !!s.on); label.textContent = typeof s.text === 'string' ? s.text : (o.label || '');
            if (o.toggle) setAttr(node, 'aria-checked', s.on ? 'true' : 'false');
            const want = typeof s.icon === 'string' ? s.icon : o.icon;   // the glyph swaps only when the name changes
            if (want !== iconName) { iconName = want; if (icon) icon.remove(); icon = iconNode(want); if (icon) node.insertBefore(icon, label); }
        }
        return bound(finish(node, set), o.input ? bindPress(node, o.input) : null);
    }

    function slider(opts) {
        const o = opts || {}, lang = langOf(o), vertical = o.variant === 'v';
        const node = root('div', 'hd-slider', withVariant(o, ['h', 'v'], 'h'), { role: 'slider', tabindex: 0, 'aria-orientation': vertical ? 'vertical' : 'horizontal', 'aria-label': o.label });
        const track = el('span', { class: 'hd-slider-track', 'aria-hidden': 'true' }, [el('span', { class: 'hd-slider-fill' }), el('span', { class: 'hd-slider-thumb' })]);
        node.appendChild(track);
        const lp = lockPaint(s => {
            applyState(node, s, { interactive: true });
            writeValue(node, s, { min: o.min, max: o.max, decimals: s.decimals ?? o.decimals, lang, unit: s.unit ?? o.unit });
        });
        const show = (v, lock) => lp.show({ value: v }, lock), shown = () => numberOf(lp.state().value);
        return bound(finish(node, lp.set), o.input ? bindLinear(node, () => track.getBoundingClientRect(), vertical ? 'y' : 'x', o.input, show, shown) : null);
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
        const lp = lockPaint(s => {
            applyState(node, s, { interactive: true });
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
        });
        const show = (v, lock) => lp.show({ value: v }, lock), shown = () => numberOf(lp.state().value);
        return bound(finish(node, lp.set), o.input ? bindDial(node, o.input, start, sweep, show, shown) : null);
    }

    function stepper(opts) {
        const o = opts || {}, lang = langOf(o);
        const node = root('div', 'hd-stepper', sized(o), { role: 'group', 'aria-label': o.label });
        const button = (dir, name, label) => el('button', { class: 'hd-step', type: 'button', 'data-dir': dir, 'aria-label': label }, iconNode(name));
        const dec = button(-1, 'minus', o.decLabel || String.fromCodePoint(0x2212)), inc = button(1, 'plus', o.incLabel || '+');
        const num = E.prim.numeral({ chars: 4, unit: o.unit, decimals: o.decimals, lang, size: o.size });
        const spin = el('span', { class: 'hd-step-value', role: 'spinbutton', tabindex: 0, 'aria-label': o.label }, num.el);
        node.appendChild(dec); node.appendChild(spin); node.appendChild(inc);
        const lp = lockPaint(s => {
            applyState(node, s, { interactive: true, targets: [spin] }); applyState(node, s, { interactive: true, targets: [dec, inc], readonlyAria: false });
            const unit = s.unit ?? o.unit, decimals = s.decimals ?? o.decimals, w = writeValue(spin, s, { min: o.min, max: o.max, decimals, lang, unit });
            num.set({ value: w.v, decimals, unit });
            if (w.v !== null && w.v <= w.min) setAttr(dec, 'aria-disabled', true);
            if (w.v !== null && w.v >= w.max) setAttr(inc, 'aria-disabled', true);
        });
        const shown = () => numberOf(lp.state().value);
        function bindStep(input) {
            const I = inputApi(), r = () => input.range(); setAttr(node, 'data-hd-input', 'step');
            const to = v => { lp.show({ value: v }, false); input.onStep(v); };
            const step = dir => { const v = I.keyValue(dir > 0 ? 'ArrowUp' : 'ArrowDown', shown(), r()); if (v !== null && v !== shown()) to(v); };
            const offs = [[dec, -1], [inc, 1]].map(([b, dir]) => I.repeat(b, Object.assign(gate(input), { onStep: () => step(dir) })));
            offs.push(I.keys(spin, Object.assign(gate(input), { range: r, get: shown, onKey: to })));
            return all(offs);
        }
        return bound(finish(node, lp.set), o.input ? bindStep(o.input) : null);
    }

    function segmented(opts) {
        const o = opts || {}, list = (Array.isArray(o.options) ? o.options : []).filter(x => x && typeof x.id === 'string');
        const node = root('div', 'hd-seg', sized(o), { role: 'radiogroup', 'aria-label': o.label });
        node.appendChild(el('span', { class: 'hd-seg-indicator', 'aria-hidden': 'true' }));
        const buttons = list.map(opt => { const icon = iconNode(opt.icon);
            const b = el('button', { class: 'hd-seg-opt', type: 'button', role: 'radio', 'data-hd-option': opt.id, 'aria-checked': 'false' }, [icon, el('span', { class: 'hd-seg-label', text: opt.label || '' })]);
            node.appendChild(b); return b; });
        if (o.rotary) node.appendChild(el('span', { class: 'hd-seg-rotary', 'aria-hidden': 'true' }, el('span', { class: 'hd-seg-rotary-pointer' })));
        let idx = -1;
        function render(index) {
            idx = index; setVars(node, { '--hd-index': index });
            buttons.forEach((b, i) => { setAttr(b, 'aria-checked', i === index ? 'true' : 'false'); setAttr(b, 'tabindex', i === (index >= 0 ? index : 0) ? 0 : -1); });
        }
        setVars(node, { '--hd-count': list.length }); render(-1);
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true });
            render(list.findIndex(x => x.id === s.value));
        }
        // Radiogroup keys (APG): Arrow wraps, Home/End jump; the pointer path picks now (false), the key path trails 400 ms in the consumer (true).
        function bindPick(input) {
            const I = inputApi(), life = E.core.bag(), n = list.length; setAttr(node, 'data-hd-input', 'pick');
            const offs = buttons.map((b, i) => I.press(b, Object.assign(gate(input), { onActivate: () => { render(i); input.onPick(list[i].id, false); } })));
            life.on(node, 'keydown', ev => {
                if (!plain(ev) || !n) return;
                const at = buttons.indexOf(ev.target), from = at >= 0 ? at : Math.max(0, idx);
                const next = ev.key === 'ArrowRight' || ev.key === 'ArrowDown' ? (from + 1) % n : ev.key === 'ArrowLeft' || ev.key === 'ArrowUp' ? (from + n - 1) % n : ev.key === 'Home' ? 0 : ev.key === 'End' ? n - 1 : -1;
                if (next < 0) return;
                stop(ev); if (!open(input, ev)) return;
                buttons[next].focus(); if (next === idx) return;
                render(next); input.onPick(list[next].id, true);
            });
            return () => { all(offs)(); life.dispose(); };
        }
        return bound(finish(node, set), o.input ? bindPick(o.input) : null);
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
        const input = o.input, node = root('div', 'hd-wheel', sized(o), input ? { role: 'slider', tabindex: 0, 'aria-label': o.label, 'aria-valuemin': 0, 'aria-valuemax': 360 } : { role: 'img', 'aria-label': o.label });
        const canvas = el('canvas', { class: 'hd-wheel-canvas', width: px, height: px, 'aria-hidden': 'true' });
        node.appendChild(canvas); node.appendChild(el('span', { class: 'hd-wheel-thumb', 'aria-hidden': 'true' }));
        paintWheel(canvas, px);
        const lp = lockPaint(s => {
            applyState(node, s, input ? { interactive: true } : undefined);
            const hue = finite(s.hue) ? ((s.hue % 360) + 360) % 360 : 0, sat = finite(s.sat) ? clamp(s.sat, 0, 1) : 0, rad = hue * Math.PI / 180;
            const rgb = E.colour.hsvToRgb(hue, sat, 1).map(n => clamp(Math.round(n), 0, 255));
            setVars(node, { '--hd-wheel-x': clamp(0.5 + 0.5 * sat * Math.sin(rad), 0, 1).toFixed(4), '--hd-wheel-y': clamp(0.5 - 0.5 * sat * Math.cos(rad), 0, 1).toFixed(4), '--hd-wheel-rgb': rgb.join(' ') });
            const text = typeof s.valueText === 'string' && s.valueText ? s.valueText : '';
            if (!input) { setAttr(node, 'aria-label', text || o.label); return; }
            setAttr(node, 'aria-valuenow', Math.round(hue)); setAttr(node, 'aria-valuetext', text || (input.valueText ? input.valueText(hue, sat * 100) : Math.round(hue) + String.fromCodePoint(0xb0) + ', ' + Math.round(sat * 100) + ' %'));
        });
        // The pointer position gives {h: clockwise from 12 oclock, s: radius 0-100}; keys per the ARIA table, the hold keys run onHold.
        function bindWheel() {
            const I = inputApi(), life = E.core.bag(), rectOf = () => node.getBoundingClientRect(), r1 = n => Math.round(n * 10) / 10;
            const hs = p => { const q = p.rect || rectOf(), dx = p.x - (q.left + q.width / 2), dy = p.y - (q.top + q.height / 2), rad = q.width / 2;
                return { h: r1((Math.atan2(dx, -dy) * 180 / Math.PI + 360) % 360) % 360, s: rad > 0 ? r1(Math.min(100, Math.hypot(dx, dy) / rad * 100)) : 0 }; };
            const show = (v, lock) => lp.show({ hue: v.h, sat: v.s / 100 }, lock);
            const now = () => { const c = lp.state(); return { h: finite(c.hue) ? ((c.hue % 360) + 360) % 360 : 0, s: finite(c.sat) ? Number((clamp(c.sat, 0, 1) * 100).toFixed(2)) : 0 }; };
            let last = null; setAttr(node, 'data-hd-input', 'drag');
            const offG = I.gesture(node, Object.assign(gate(input), { rect: rectOf, onHold: input.onHold,
                onTap: p => { const v = hs(p); show(v, false); input.onCommit(v); },
                onDrag: p => { last = hs(p); show(last, true); input.onInput(last); },
                onDragEnd: () => { if (!last) return; const v = last; last = null; show(v, false); input.onCommit(v); } }));
            life.on(node, 'keydown', ev => {
                if (!plain(ev)) return;
                if (I.isHoldKey(ev)) { if (input.onHold) { stop(ev); input.onHold(ev); } return; }
                const c = now(); let h = c.h, s = c.s;
                switch (ev.key) { case 'ArrowLeft': h -= 5; break; case 'ArrowRight': h += 5; break; case 'PageDown': h -= 30; break; case 'PageUp': h += 30; break;
                    case 'ArrowDown': s -= 5; break; case 'ArrowUp': s += 5; break; case 'Home': s = 0; break; case 'End': s = 100; break; default: return; }
                stop(ev); h = ((h % 360) + 360) % 360; s = clamp(s, 0, 100);
                if (!open(input, ev) || (h === c.h && s === c.s)) return;
                show({ h, s }, false); input.onKey({ h, s }, ev.key);
            });
            return () => { offG(); life.dispose(); };
        }
        return bound(finish(node, lp.set), input ? bindWheel() : null);
    }

    function ctStrip(opts) {
        const o = opts || {}, lang = langOf(o), min = finite(o.min) ? o.min : 2200, max = finite(o.max) ? o.max : 6500;
        const node = root('div', 'hd-ct', sized(o), { role: 'slider', tabindex: 0, 'aria-label': o.label });
        node.appendChild(el('span', { class: 'hd-ct-track', 'aria-hidden': 'true' })); node.appendChild(el('span', { class: 'hd-ct-thumb', 'aria-hidden': 'true' }));
        const lp = lockPaint(s => { applyState(node, s, { interactive: true }); writeValue(node, s, { min, max, decimals: 0, grouping: false, unit: 'K', lang }); });
        const show = (v, lock) => lp.show({ value: v }, lock), shown = () => numberOf(lp.state().value);
        return bound(finish(node, lp.set), o.input ? bindLinear(node, () => node.getBoundingClientRect(), 'x', o.input, show, shown) : null);
    }

    // Combo trigger of a select (the listbox itself is E.popover). set({text, expanded, label}) plus the common state.
    function picker(opts) {
        const o = opts || {}, node = root('button', 'hd-pick', sized(o), { type: 'button', 'aria-haspopup': 'listbox', 'aria-expanded': 'false', 'aria-label': o.label });
        const label = el('span', { class: 'hd-pick-label', text: o.text || '' }); node.appendChild(label); node.appendChild(iconNode('chevron-down'));
        function set(state) {
            const s = state || {}; applyState(node, s, { interactive: true, readonlyAria: false });
            label.textContent = typeof s.text === 'string' ? s.text : (o.text || ''); setAttr(node, 'aria-expanded', s.expanded ? 'true' : 'false'); setAttr(node, 'aria-label', typeof s.label === 'string' && s.label ? s.label : o.label);
        }
        function bindPicker(input) {
            const life = E.core.bag(), off = bindPress(node, input);
            life.on(node, 'keydown', ev => { if (ev.key !== 'ArrowDown' || !plain(ev)) return; stop(ev); if (open(input, ev) && input.onActivate) input.onActivate(ev); });
            return () => { off(); life.dispose(); };
        }
        return bound(finish(node, set), o.input ? bindPicker(o.input) : null);
    }

    Object.assign(E.prim, { toggle, rocker, pushButton, slider, knob, stepper, segmented, colourWheel, ctStrip, picker });
})();
