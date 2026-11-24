// HomeDash display primitives (static builders): numeral, segReadout, arcGauge, needleGauge, bar, lamp, badge, iconBadge,
// sparkline, frame and sheet, plus E.prim.util shared with the control builders. Each builder returns {el, set(state), destroy()}
// and writes only attributes, text content and custom properties; all painting is CSS (markup contract C2).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.geometry || !E.icons) throw new Error('hd-prim-displays.js: load hd-core, hd-format, hd-icons and hd-geometry first');
    E.prim = E.prim || {};
    const { setAttr, setVars, el } = E.core, svgEl = E.core.svg, G = E.geometry, F = E.format;
    const deg = a => String(Math.round(a * 1000) / 1000) + 'deg';
    const fmtN = v => String(Math.round(v * 1000) / 1000);
    function dash(path, frac, quantize) { const f = quantize > 0 ? Math.round(frac * quantize) / quantize : frac;
        path.style.setProperty('stroke-dasharray', String(Math.round(f * 10000) / 10) + ' 1000'); }
    function applyState(node, state, opts) {
        const s = state || {}, o = opts || {}, status = ['ok', 'unavailable', 'unknown', 'error'].includes(s.status) ? s.status : 'ok';
        setAttr(node, 'data-hd-status', status); setAttr(node, 'data-hd-pending', !!s.pending); setAttr(node, 'data-hd-stale', !!s.stale);
        setAttr(node, 'data-hd-readonly', !!s.readonly); setAttr(node, 'data-hd-disabled', !!s.disabled); setAttr(node, 'data-hd-tone', ['ok', 'warn', 'crit'].includes(s.tone) ? s.tone : null);
        if (!o.interactive) return;
        const off = !!s.disabled || status === 'unavailable';
        for (const t of o.targets || [node]) { if (o.readonlyAria === false) setAttr(t, 'aria-disabled', off || !!s.readonly); else { setAttr(t, 'aria-disabled', off); setAttr(t, 'aria-readonly', !!s.readonly); } }
    }
    function writeValue(node, state, opts, aria) {
        const s = state || {}, o = opts || {}, min = typeof s.min === 'number' ? s.min : o.min ?? 0, max = typeof s.max === 'number' ? s.max : o.max ?? 100;
        const v = typeof s.value === 'number' && Number.isFinite(s.value) ? s.value : null, frac = v === null ? 0 : G.fraction(v, min, max);
        setVars(node, { '--hd-value': frac.toFixed(4) }); setAttr(node, 'data-hd-empty', v === null);
        if (aria !== false) { setAttr(node, 'aria-valuemin', String(min)); setAttr(node, 'aria-valuemax', String(max)); setAttr(node, 'aria-valuenow', v === null ? null : String(v));
            setAttr(node, 'aria-valuetext', s.valueText || F.valueText(v, { decimals: o.decimals, lang: o.lang, unit: o.unit, grouping: o.grouping })); }
        return { v, frac, min, max };
    }
    const root = (tag, cls, opts, attrs) => el(tag, Object.assign({ class: cls, 'data-hd-size': (opts && opts.size) || 'm' }, opts && opts.variant ? { 'data-hd-variant': opts.variant } : {}, attrs));
    function finish(node, set, extra) { let done = false; return Object.assign({ el: node, set, destroy() { if (done) return; done = true; node.remove(); } }, extra); }
    // Path of the last lit segment of a quantized arc (empty when none is lit): segment k spans a(k-1)..a(k), a(i) = start + i * sweep / N.
    function leadArc(start, sweep, quantize, frac) { const k = Math.round(frac * quantize); if (!(k > 0)) return ''; const step = sweep / quantize; return G.arc(100, 100, 88, start + (k - 1) * step, step); }
    const util = Object.freeze({ applyState, writeValue, root, finish, deg, dash, fmtN, leadArc });
    E.prim.util = util;

    const TONES = ['ok', 'warn', 'crit'], BADGE_TONES = ['ok', 'warn', 'crit', 'info', 'neutral'], DASH = F.DASH, DEG = String.fromCodePoint(0xb0);
    const TREND = { up: String.fromCodePoint(0x2191), down: String.fromCodePoint(0x2193), flat: String.fromCodePoint(0x2192) };
    const langOf = o => (o && o.lang) || 'en';
    const finite = v => typeof v === 'number' && Number.isFinite(v);
    // Swaps one icon svg inside parent (before `before`), only when the name changes.
    function iconSlot(parent, before, first) { let cur = null, curName;
        const put = name => { if (name === curName) return; curName = name; if (cur) { cur.remove(); cur = null; }
            if (name) { cur = E.icons.icon(name); parent.insertBefore(cur, before || null); } };
        put(first); return put; }
    function toneOf(state, zones, v) {
        if (TONES.includes(state.tone)) return state.tone;
        if (finite(v)) for (const z of zones) if (v >= z.from && v <= z.to && z.tone) return z.tone;
        return null; }
    const validZones = z => (Array.isArray(z) ? z.filter(x => x && finite(x.from) && finite(x.to)) : []);

    function numeral(opts) {
        const o = opts || {}, lang = langOf(o);
        const node = root('span', 'hd-numeral', o), trendIcon = !!o.trendIcon;
        const int = el('span', { class: 'hd-num-int' }), frac = el('span', { class: 'hd-num-frac' }), degEl = el('span', { class: 'hd-num-deg' });
        const unitEl = el('span', { class: 'hd-num-unit' }), trendEl = el('span', { class: 'hd-num-trend', 'aria-hidden': 'true' });
        node.appendChild(int); node.appendChild(frac); node.appendChild(degEl); node.appendChild(unitEl); node.appendChild(trendEl);
        const trendSlot = trendIcon ? iconSlot(trendEl, null, null) : null;
        setVars(node, { '--hd-num-chars': String(o.chars || 4) });
        function set(state) {
            const s = state || {}; applyState(node, s);
            if (typeof s.text === 'string') {
                int.textContent = s.text; frac.textContent = ''; degEl.textContent = ''; unitEl.textContent = ''; setAttr(node, 'data-hd-empty', false);
            } else {
                const p = F.split(s.value, { decimals: s.decimals ?? o.decimals, lang, unit: s.unit ?? o.unit });
                int.textContent = p.int; frac.textContent = p.frac; unitEl.textContent = p.unit; setAttr(node, 'data-hd-empty', p.empty);
                degEl.textContent = (s.degree ?? o.degree) ? DEG : '';
            }
            const glyph = Object.prototype.hasOwnProperty.call(TREND, s.trend) ? TREND[s.trend] : '';
            if (trendSlot) trendSlot(glyph ? 'trend-' + s.trend : null); else trendEl.textContent = glyph;
            setAttr(trendEl, 'data-trend', glyph ? s.trend : null);
        }
        return finish(node, set);
    }

    function segReadout(opts) {
        const o = opts || {}, lang = langOf(o), digits = Math.max(1, Math.floor(o.digits || 4)), W = 12, H = 20, paths = G.segmentPaths(W, H, 2.4, 0.6);
        const node = root('span', 'hd-seg7', o, { role: 'img', 'aria-label': o.label });
        const svg = svgEl('svg', { class: 'hd-seg7-svg', viewBox: '0 0 ' + (16 * digits) + ' 20', 'aria-hidden': 'true' });
        const cells = [];
        for (let i = 0; i < digits; i++) {
            const g = svgEl('g', { class: 'hd-seg7-digit', 'data-index': i, transform: 'translate(' + (16 * i) + ' 0)' }), segs = {};
            for (const k of ['a', 'b', 'c', 'd', 'e', 'f', 'g']) { segs[k] = svgEl('path', { class: 'hd-seg7-seg', 'data-seg': k, d: paths[k] }); g.appendChild(segs[k]); }
            const dp = svgEl('path', { class: 'hd-seg7-dp', d: paths.dp }); g.appendChild(dp); svg.appendChild(g); cells.push({ segs, dp });
        }
        const unitEl = el('span', { class: 'hd-seg7-unit', text: o.unit || '' });
        node.appendChild(svg); node.appendChild(unitEl);
        function set(state) {
            const s = state || {}; applyState(node, s);
            const v = finite(s.value) ? s.value : null, unit = s.unit ?? o.unit, decimals = s.decimals ?? o.decimals;
            const text = typeof s.text === 'string' ? s.text : v === null ? '' : F.number(v, { decimals, lang, grouping: false });
            const glyphs = G.sevenSeg(text).slice(-digits), offset = digits - glyphs.length;
            cells.forEach((c, i) => { const gl = i >= offset ? glyphs[i - offset] : null;
                for (const k of Object.keys(c.segs)) setAttr(c.segs[k], 'data-lit', !!gl && gl.segs.includes(k)); setAttr(c.dp, 'data-lit', !!gl && gl.dp); });
            setAttr(node, 'data-hd-empty', v === null && typeof s.text !== 'string'); unitEl.textContent = unit || '';
            setAttr(node, 'aria-label', typeof s.text === 'string' && s.text ? s.text : F.valueText(v, { decimals, lang, unit }));
        }
        return finish(node, set);
    }

    // Shared gauge frame: readout numeral and min/max labels.
    function gaugeParts(o, lang) {
        const num = o.readout === 'seg7' ? segReadout({ digits: 4, decimals: o.decimals, unit: o.unit, lang }) : numeral({ chars: o.chars || 4, unit: o.unit, decimals: o.decimals, lang });
        return { num, readout: el('span', { class: 'hd-gauge-readout' }, num.el), minEl: el('span', { class: 'hd-gauge-min' }), maxEl: el('span', { class: 'hd-gauge-max' }) };
    }
    function zonePaths(zones, r, start, sweep, min, max) {
        return zones.map(z => ({ z, fa: G.fraction(z.from, min, max), fb: G.fraction(z.to, min, max) })).filter(x => x.fb > x.fa)
            .map(x => svgEl('path', { class: 'hd-gauge-zone', 'data-tone': x.z.tone, d: G.arc(100, 100, r, start + sweep * x.fa, sweep * (x.fb - x.fa)) }));
    }

    function arcGauge(opts) {
        const o = opts || {}, lang = langOf(o), min0 = finite(o.min) ? o.min : 0, max0 = finite(o.max) ? o.max : 100, quantize = o.quantize > 0 ? o.quantize : 0, zones = validZones(o.zones);
        const node = root('div', 'hd-gauge', o, { 'data-hd-gauge': 'arc', role: 'meter', 'aria-label': o.label, 'data-hd-quantize': quantize > 0 ? quantize : null });
        const svg = svgEl('svg', { class: 'hd-gauge-svg', viewBox: '0 0 200 ' + (o.crop || 200), 'aria-hidden': 'true' });
        const full = G.arc(100, 100, 88, -135, 270);
        svg.appendChild(svgEl('path', { class: 'hd-gauge-track', pathLength: 1000, d: full }));
        for (const p of zonePaths(zones, 76, -135, 270, min0, max0)) svg.appendChild(p);
        const glowEl = o.glow ? svgEl('path', { class: 'hd-gauge-glow', pathLength: 1000, d: full }) : null; if (glowEl) svg.appendChild(glowEl);
        const arcEl = svgEl('path', { class: 'hd-gauge-arc', pathLength: 1000, d: full }); svg.appendChild(arcEl);
        const zoneEl = o.zoneOverlay ? svgEl('path', { class: 'hd-gauge-arc-zone', pathLength: 1000, d: full }) : null; if (zoneEl) svg.appendChild(zoneEl);
        const leadEl = o.lead && quantize > 0 ? svgEl('path', { class: 'hd-gauge-lead', d: '' }) : null; if (leadEl) svg.appendChild(leadEl);
        svg.appendChild(svgEl('g', { class: 'hd-gauge-cap' }, svgEl('circle', { class: 'hd-gauge-capdot', cx: 100, cy: 12, r: 7 })));
        if (o.gaps > 0) svg.appendChild(svgEl('path', { class: 'hd-gauge-gaps', pathLength: 1000, d: full }));
        const parts = gaugeParts(o, lang);
        node.appendChild(svg); node.appendChild(parts.readout); node.appendChild(parts.minEl); node.appendChild(parts.maxEl);
        function set(state) {
            const s = state || {}; applyState(node, s);
            const w = writeValue(node, s, { min: o.min, max: o.max, decimals: o.decimals, lang, unit: s.unit ?? o.unit });
            setVars(node, { '--hd-angle': deg(-135 + w.frac * 270) }); dash(arcEl, w.frac, quantize);
            if (glowEl) dash(glowEl, w.frac, quantize); if (zoneEl) dash(zoneEl, w.frac, quantize);
            if (leadEl) leadEl.setAttribute('d', leadArc(-135, 270, quantize, w.frac));
            setAttr(node, 'data-hd-tone', toneOf(s, zones, w.v));
            parts.num.set({ value: w.v, unit: s.unit ?? o.unit, decimals: s.decimals ?? o.decimals });
            parts.minEl.textContent = F.number(w.min, { lang }); parts.maxEl.textContent = F.number(w.max, { lang });
        }
        return finish(node, set);
    }

    function needleGauge(opts) {
        const o = opts || {}, lang = langOf(o), min0 = finite(o.min) ? o.min : 0, max0 = finite(o.max) ? o.max : 100, zones = validZones(o.zones);
        const node = root('div', 'hd-gauge', o, { 'data-hd-gauge': 'needle', role: 'meter', 'aria-label': o.label });
        if (o.face) node.appendChild(el('span', { class: 'hd-gauge-face', 'aria-hidden': 'true' }, [el('span', { class: 'hd-gauge-bezel' }), el('span', { class: 'hd-gauge-glass' })]));
        const svg = svgEl('svg', { class: 'hd-gauge-svg', viewBox: '0 0 200 ' + (o.crop || 200), 'aria-hidden': 'true' });
        const majors = o.majors ?? 10, minors = o.minors ?? 50;
        svg.appendChild(svgEl('path', { class: 'hd-gauge-ticks', 'data-major': 'true', d: G.ticks(100, 100, 72, 88, -120, 240, majors) }));
        svg.appendChild(svgEl('path', { class: 'hd-gauge-ticks', 'data-minor': 'true', d: G.ticks(100, 100, 80, 88, -120, 240, minors) }));
        for (const p of zonePaths(zones, 84, -120, 240, min0, max0)) svg.appendChild(p);
        const count = Math.max(0, Math.min(6, Math.floor(o.numerals ?? 6)));
        for (let i = 0; i < count; i++) {
            const t = i / Math.max(1, count - 1), pt = G.point(100, 100, 54, -120 + 240 * t);
            svg.appendChild(svgEl('text', { class: 'hd-gauge-numeral', x: fmtN(pt[0]), y: fmtN(pt[1]), text: F.number(min0 + t * (max0 - min0), { lang }) }));
        }
        const needleD = G.needle(100, 100, 88, 24, 6.4, 1.6);
        if (o.needleFx) svg.appendChild(svgEl('g', { class: 'hd-gauge-needle-shadow' }, svgEl('path', { d: needleD })));
        svg.appendChild(svgEl('g', { class: 'hd-gauge-needle' }, [o.needleFx ? svgEl('path', { class: 'hd-gauge-needle-glow', d: needleD }) : null,
            svgEl('path', { class: 'hd-gauge-needle-shape', d: needleD }), svgEl('circle', { class: 'hd-gauge-hub', cx: 100, cy: 100, r: 9 })]));
        const parts = gaugeParts(o, lang);
        node.appendChild(svg); node.appendChild(parts.readout); node.appendChild(parts.minEl); node.appendChild(parts.maxEl);
        let prev;
        function set(state) {
            const s = state || {}; applyState(node, s);
            const w = writeValue(node, s, { min: o.min, max: o.max, decimals: o.decimals, lang, unit: s.unit ?? o.unit });
            if (G.needleChanged(prev, w.v, w.min, w.max)) { prev = w.v; setVars(node, { '--hd-angle': deg(-120 + w.frac * 240) }); }
            setAttr(node, 'data-hd-tone', toneOf(s, zones, w.v));
            parts.num.set({ value: w.v, unit: s.unit ?? o.unit, decimals: s.decimals ?? o.decimals });
            parts.minEl.textContent = F.number(w.min, { lang }); parts.maxEl.textContent = F.number(w.max, { lang });
        }
        return finish(node, set);
    }

    function bar(opts) {
        const o = opts || {}, lang = langOf(o), min0 = finite(o.min) ? o.min : 0, max0 = finite(o.max) ? o.max : 100, zones = validZones(o.zones);
        const node = root('div', 'hd-bar', Object.assign({}, o, { variant: o.variant || 'h' }), { role: 'meter', 'aria-label': o.label });
        const track = el('span', { class: 'hd-bar-track', 'aria-hidden': 'true' });
        for (const z of zones) { const fa = G.fraction(z.from, min0, max0), fb = G.fraction(z.to, min0, max0);
            track.appendChild(el('span', { class: 'hd-bar-zone', 'data-tone': z.tone, vars: { '--hd-from': fa.toFixed(4), '--hd-to': fb.toFixed(4) } })); }
        track.appendChild(el('span', { class: 'hd-bar-fill' })); node.appendChild(track);
        const num = o.readout ? numeral({ chars: o.chars || 4, unit: o.unit, decimals: o.decimals, lang }) : null;
        if (num) node.appendChild(el('span', { class: 'hd-bar-readout' }, num.el));
        function set(state) {
            const s = state || {}; applyState(node, s);
            const w = writeValue(node, s, { min: o.min, max: o.max, decimals: o.decimals, lang, unit: s.unit ?? o.unit });
            setAttr(node, 'data-hd-tone', toneOf(s, zones, w.v));
            if (num) num.set({ value: w.v, unit: s.unit ?? o.unit, decimals: s.decimals ?? o.decimals });
        }
        return finish(node, set);
    }

    function lamp(opts) {
        const o = opts || {}, node = root('span', 'hd-lamp', o, { role: 'img', 'aria-label': o.label });
        node.appendChild(el('span', { class: 'hd-lamp-dot', 'aria-hidden': 'true' })); node.appendChild(el('span', { class: 'hd-lamp-halo', 'aria-hidden': 'true' }));
        function set(state) {
            const s = state || {}; applyState(node, s);
            setAttr(node, 'data-hd-on', !!s.on); setAttr(node, 'aria-label', typeof s.text === 'string' && s.text ? s.text : o.label);
        }
        return finish(node, set);
    }

    function badge(opts) {
        const o = opts || {}, node = root('span', 'hd-badge', o, { 'data-tone': o.tone && BADGE_TONES.includes(o.tone) ? o.tone : 'neutral' });
        const text = el('span', { class: 'hd-badge-text', text: o.text || '' }); node.appendChild(text);
        const slot = iconSlot(node, text, o.icon);
        function set(state) {
            const s = state || {}; applyState(node, s);
            text.textContent = s.text == null ? (o.text || '') : String(s.text);
            setAttr(node, 'data-tone', BADGE_TONES.includes(s.tone) ? s.tone : o.tone && BADGE_TONES.includes(o.tone) ? o.tone : 'neutral');
            if (s.icon !== undefined) slot(s.icon || null);
        }
        return finish(node, set);
    }

    function iconBadge(opts) {
        const o = opts || {}, node = root('span', 'hd-iconbadge', o, { 'aria-hidden': 'true' }), slot = iconSlot(node, null, o.icon || 'entity');
        function set(state) { const s = state || {}; applyState(node, s); if (s.icon !== undefined) slot(s.icon || 'entity'); }
        return finish(node, set);
    }

    function sparkline(opts) {
        const o = opts || {}; let w = o.w || 120, h = o.h || 40, points = [];
        const node = svgEl('svg', { class: 'hd-spark', role: 'img', 'aria-label': o.label, viewBox: '0 0 ' + w + ' ' + h, width: w, height: h, 'data-hd-size': o.size || 'm' });
        const area = o.area === false ? null : svgEl('path', { class: 'hd-spark-area' }), line = svgEl('path', { class: 'hd-spark-line' });
        const last = svgEl('circle', { class: 'hd-spark-last', r: 3, hidden: 'true' });
        const fill = area && o.fade ? svgEl('svg', { class: 'hd-spark-fill', 'aria-hidden': 'true', viewBox: '0 0 ' + w + ' ' + h, width: w, height: h }, area) : null;
        const cols = Array.isArray(o.grid) ? Math.floor(o.grid[0]) : 0, rows = Array.isArray(o.grid) ? Math.floor(o.grid[1]) : 0;
        const grid = cols > 0 && rows > 0 ? svgEl('path', { class: 'hd-spark-grid', d: '' }) : null;
        const halo = o.halo ? svgEl('circle', { class: 'hd-spark-halo', r: 6, hidden: 'true' }) : null;
        if (grid) node.appendChild(grid); if (area && !fill) node.appendChild(area); node.appendChild(line); if (halo) node.appendChild(halo); node.appendChild(last);
        // Interior grid lines only, snapped to half pixels: vertical at round(w*i/cols)+0.5, horizontal at round(h*j/rows)+0.5.
        function gridPath() { let d = '';
            for (let i = 1; i < cols; i++) d += 'M' + (Math.round(w * i / cols) + 0.5) + ' 0V' + h;
            for (let j = 1; j < rows; j++) d += 'M0 ' + (Math.round(h * j / rows) + 0.5) + 'H' + w;
            return d; }
        function resize(nw, nh) { const wide = nw !== w, tall = nh !== h; w = nw; h = nh;
            for (const t of fill ? [node, fill] : [node]) { setAttr(t, 'viewBox', '0 0 ' + w + ' ' + h); if (wide) setAttr(t, 'width', w); if (tall) setAttr(t, 'height', h); } }
        function set(state) {
            const s = state || {}; applyState(node, s);
            if (finite(s.w) && finite(s.h) && (s.w !== w || s.h !== h)) resize(s.w, s.h);
            else if (finite(s.w) && s.w !== w) resize(s.w, h);
            else if (finite(s.h) && s.h !== h) resize(w, s.h);
            if (Array.isArray(s.points)) points = s.points;
            const g = G.sparkline(points, { w, h });
            line.setAttribute('d', g.line); if (area) area.setAttribute('d', g.area); if (grid) grid.setAttribute('d', gridPath());
            for (const c of halo ? [last, halo] : [last]) { if (g.last) { setAttr(c, 'cx', fmtN(g.last.x)); setAttr(c, 'cy', fmtN(g.last.y)); } setAttr(c, 'hidden', !g.last); }
            setAttr(node, 'data-hd-empty', !g.last);
            setAttr(node, 'aria-label', typeof s.summary === 'string' && s.summary ? s.summary : o.label);
        }
        const out = finish(node, set, fill ? { fill } : undefined);
        if (fill) { const base = out.destroy; out.destroy = () => { base(); fill.remove(); }; }
        return out;
    }

    function frame(opts) {
        const o = opts || {}, node = root('div', 'hd-frame', o), titleId = E.core.uid('hd-t');
        const iconHost = el('span', { class: 'hd-icon' }), title = el('span', { class: 'hd-title', id: titleId, text: o.title || '' }), sub = el('span', { class: 'hd-sub', text: o.sub || '' });
        const head = el('div', { class: 'hd-head' }, [iconHost, title, sub]), body = el('div', { class: 'hd-body' }), foot = el('div', { class: 'hd-foot', hidden: 'true' });
        const stateText = el('span', { class: 'hd-state-text' });
        if (typeof o.foot === 'string' && o.foot) { foot.textContent = o.foot; foot.removeAttribute('hidden'); }
        else if (o.foot && o.foot.nodeType) { foot.appendChild(o.foot); foot.removeAttribute('hidden'); }
        node.appendChild(head); node.appendChild(body); node.appendChild(foot); node.appendChild(el('div', { class: 'hd-state' }, stateText));
        if (o.more) node.appendChild(el('button', { class: 'hd-more', type: 'button', 'aria-label': o.more }));
        const slot = iconSlot(iconHost, null, o.icon || null);
        function set(state) {
            const s = state || {}; applyState(node, s);
            if (s.title !== undefined) title.textContent = String(s.title); if (s.sub !== undefined) sub.textContent = String(s.sub);
            if (s.stateText !== undefined) stateText.textContent = String(s.stateText); if (s.icon !== undefined) slot(s.icon || null);
        }
        return finish(node, set, { body, head, foot, titleId });
    }

    function sheet(opts) {
        const o = opts || {}, titleId = E.core.uid('hd-t'), layer = root('div', 'hd-sheet-layer', o, { hidden: 'true' });
        const title = el('h2', { class: 'hd-sheet-title', id: titleId, text: o.title || '' }), body = el('div', { class: 'hd-sheet-body' }), foot = el('div', { class: 'hd-sheet-foot' });
        const dlg = el('div', { class: 'hd-sheet', role: 'dialog', 'aria-modal': 'true', 'data-kind': o.kind, 'aria-labelledby': titleId }, [el('div', { class: 'hd-sheet-head' }, title), body, foot]);
        layer.appendChild(dlg);
        function set(state) {
            const s = state || {}; applyState(layer, s);
            if (s.open !== undefined) setAttr(layer, 'hidden', !s.open);
            if (s.title !== undefined) title.textContent = String(s.title);
        }
        return finish(layer, set, { body, foot });
    }

    Object.assign(E.prim, { numeral, segReadout, arcGauge, needleGauge, bar, lamp, badge, iconBadge, sparkline, frame, sheet });
})();
