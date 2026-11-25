// HomeDash gauge, temperature and sensor widgets. Heroes: gauge/needle (H5), gauge/arc (H6, Tactile substitutes the needle gauge),
// temperature/numeral (H7), sensor/sparkline (H8) and sensor/numeral (the H7 builder without degree and trend). The other variants are placeholders.
// These widgets are read-only displays: they check no capability, so the state line never shows the "blocked" text.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.prim || !E.widgets) throw new Error('hd-w-sensor.js: load hd-core, hd-format, the primitives and hd-widgets first');
    const { el, setAttr } = E.core, F = E.format, MID_DOT = ' ' + String.fromCodePoint(0xb7) + ' ';
    const DEG = String.fromCodePoint(0xb0), DEG_C = DEG + 'C', HOST_WORDS = ['unavailable', 'unknown', 'error'];
    const MIN30 = 30 * 60000, TREND_WORDS = { up: 'homedash.trend.rising', down: 'homedash.trend.falling', flat: 'homedash.trend.steady' };

    const hostWord = e => !e || e.available === false || HOST_WORDS.includes(e.status);
    const numberOf = v => (v === null || v === undefined || v === '' ? null : Number.isFinite(Number(v)) ? Number(v) : null);
    // Status text per module rule 5: null lets the host words win (no entity, unavailable, unknown, error or pending).
    function statusText(api, e, hero) {
        if (hostWord(e) || e.pending) return null;
        if (e.readonly) return hero ? hero + MID_DOT + api.t('homedash.state.readonly') : api.t('homedash.state.readonly');
        if (e.stale) { const old = E.widgets.heroStatus.staleText(api, e); if (old) return old; }
        return hero || null;
    }
    // SCR-M2-3: a widget's options.decimals (integer 0-3) sets the readout precision of gauges and sensors; anything else keeps the default (0).
    const decimalsOf = (api, dflt) => { const d = api.widget.options && api.widget.options.decimals; return Number.isInteger(d) && d >= 0 && d <= 3 ? d : dflt; };
    const flip = node => setAttr(node, 'data-hd-tick', node.getAttribute('data-hd-tick') === 'a' ? 'b' : 'a');
    const valid = pts => (Array.isArray(pts) ? pts.filter(p => Array.isArray(p) && Number.isFinite(p[0]) && Number.isFinite(p[1])) : []);
    function extent(points) {
        const vs = valid(points).map(p => p[1]);
        return vs.length ? { min: Math.min.apply(null, vs), max: Math.max.apply(null, vs) } : null;
    }
    // Trend over the last 30 minutes of the series itself (window ending at the last point's own time, never the wall clock).
    function trendOf(points) {
        const pts = valid(points); if (pts.length < 2) return 'flat';
        const last = pts[pts.length - 1], from = pts.find(p => p[0] >= last[0] - MIN30 && p[0] < last[0]);
        if (!from) return 'flat';
        const perHour = (last[1] - from[1]) / ((last[0] - from[0]) / 3600000);
        return perHour > 0.3 ? 'up' : perHour < -0.3 ? 'down' : 'flat';
    }
    function history(api, apply) {
        const p = api.history('primary', '24h');
        if (p && typeof p.then === 'function') p.then(s => { if (!api.signal.aborted && s) apply(s); }, () => {});
    }

    // ---- Gauges (H5, H6) ----
    function zonesOf(api) {
        const z = api.widget.options && api.widget.options.zones;
        return Array.isArray(z) ? z.filter(x => x && Number.isFinite(x.from) && Number.isFinite(x.to) && (x.tone === 'warn' || x.tone === 'crit')) : [];
    }
    function gauge(api, kind) {
        const inst = api.family === 'instrument', tact = api.family === 'tactile', glass = api.family === 'glass', o = api.widget.options || {};
        const substitute = kind === 'arc' && tact;
        const min = substitute ? 0 : Number.isFinite(o.min) ? o.min : 0, max = substitute ? 100 : Number.isFinite(o.max) ? o.max : 100;
        const zones = substitute ? [{ from: 80, to: 92, tone: 'warn' }, { from: 92, to: 100, tone: 'crit' }] : zonesOf(api), unit = substitute ? '%' : o.unit;
        const readout = inst ? 'seg7' : 'numeral', label = api.widget.title, lang = api.lang, dec = decimalsOf(api, 0);
        const prim = kind === 'needle' || substitute
            ? api.prim.needleGauge({ label, min, max, unit, decimals: dec, zones, majors: 10, minors: 50, numerals: 6, crop: tact ? 200 : 168, face: tact, needleFx: tact || inst, readout, lang })
            : api.prim.arcGauge({ label, min, max, unit, decimals: dec, zones, quantize: inst ? 40 : 0, crop: 172, glow: glass || inst, gaps: inst ? 40 : 0, lead: inst, zoneOverlay: true, readout, lang });
        api.body.appendChild(prim.el);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        const zoneTone = v => { if (v === null) return null; const z = zones.find(x => v >= x.from && v <= x.to); return z ? z.tone : null; };
        return {
            update(view) {
                const e = view.entities.primary, v = e ? numberOf(e.value) : null, shown = hostWord(e) ? null : v;
                api.setTone(e && e.tone ? e.tone : zoneTone(shown));
                prim.set({ value: shown, unit: substitute ? '%' : (e && e.unit) || unit, tone: e && e.tone ? e.tone : null, status: e ? e.status : 'unavailable', pending: !!(e && e.pending),
                    stale: !!(e && e.stale), readonly: !!(e && e.readonly) });
                api.setStatus(statusText(api, e, null));
            },
            destroy() { prim.destroy(); }
        };
    }

    // ---- Numeral readout shared by temperature/numeral (H7), sensor/numeral and the H8 head ----
    // cfg: {chars, decimals, degree, trend}. Returns the primitive plus a render(state) that flips data-hd-tick when the displayed value changes.
    function readoutOf(api, cfg) {
        const inst = api.family === 'instrument', label = api.widget.title, lang = api.lang;
        const prim = inst ? api.prim.segReadout({ digits: cfg.chars, decimals: cfg.decimals, unit: cfg.degree ? DEG_C : '', label, lang })
            : api.prim.numeral({ chars: cfg.chars, decimals: cfg.decimals, unit: cfg.degree ? 'C' : '', degree: cfg.degree, trendIcon: cfg.trend, lang });
        return { prim, inst };
    }
    function unitFor(cfg, inst, e) {
        const raw = e && typeof e.unit === 'string' ? e.unit : '';
        if (cfg.degree) { const u = raw.replace(DEG, '') || 'C'; return inst ? DEG + u : u; }
        return raw;
    }
    function readoutText(api, cfg, unit, v, trend) {
        const text = F.valueText(v, { decimals: cfg.decimals, lang: api.lang, unit: cfg.degree ? DEG + unit : unit });
        return trend ? text + ', ' + api.t(TREND_WORDS[trend]) : text;
    }

    // ---- H7 temperature/numeral and sensor/numeral ----
    function tread(api, isTemperature) {
        const cfg = isTemperature ? { chars: 4, decimals: 1, degree: true, trend: true } : { chars: 5, decimals: decimalsOf(api, 0), degree: false, trend: false };
        const { prim, inst } = readoutOf(api, cfg);
        // The visible numeral is aria-hidden and the sr-only span speaks value, unit and trend; the seven-segment readout is role=img with its own label.
        const sr = inst ? null : el('span', { class: 'hd-sr-only' });
        if (sr) setAttr(prim.el, 'aria-hidden', 'true');
        const vfd = el('div', { class: 'hd-vfd' }, sr ? [prim.el, sr] : [prim.el]), node = el('div', { class: 'hd-tread' }, vfd);
        api.body.appendChild(node);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        let last = null, lastView = null, range = null, trend = null, lastShown;
        function render() {
            const e = lastView && lastView.entities.primary, v = e && !hostWord(e) ? numberOf(e.value) : null, unit = unitFor(cfg, inst, e);
            const t = isTemperature && v !== null ? trend : null;
            api.setTone(e && e.tone ? e.tone : null);
            prim.set({ value: v, unit, trend: t, status: e ? e.status : 'unavailable', stale: !!(e && e.stale), readonly: !!(e && e.readonly) });
            if (sr) sr.textContent = readoutText(api, cfg, cfg.degree ? unit.replace(DEG, '') : unit, v, t);
            if (lastShown !== undefined && lastShown !== v) flip(node);
            lastShown = v;
            const rangeText = isTemperature && range ? api.t('homedash.temperature.range', { min: F.valueText(range.min, { decimals: 1, lang: api.lang }), max: F.valueText(range.max, { decimals: 1, lang: api.lang, unit: DEG_C }) }) : null;
            api.setStatus(statusText(api, e, rangeText));
        }
        if (isTemperature) history(api, s => { range = extent(s.points); trend = trendOf(s.points); if (lastView) render(); });
        return { update(view) { lastView = view; render(); }, destroy() { prim.destroy(); node.remove(); } };
    }

    // ---- H8 sensor/sparkline ----
    function sparkline(api) {
        const inst = api.family === 'instrument', tact = api.family === 'tactile', glass = api.family === 'glass';
        const cfg = { chars: 5, decimals: decimalsOf(api, 0), degree: false, trend: false }, { prim } = readoutOf(api, cfg);
        const spark = api.prim.sparkline({ label: api.widget.title, area: true, fade: glass, grid: tact || inst ? [4, 3] : null, halo: true });
        // Same rule as H7: aria-hidden numeral plus an sr-only reading, none for the seven-segment readout.
        const sr = inst ? null : el('span', { class: 'hd-sr-only' });
        if (sr) setAttr(prim.el, 'aria-hidden', 'true');
        api.body.appendChild(prim.el);
        if (sr) api.body.appendChild(sr);
        if (spark.fill) api.body.appendChild(spark.fill);
        api.body.appendChild(spark.el);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        let lastView = null, hist = [], dims = null, lastShown, loaded = false;
        // SCR-M2-2: after the history arrived, a fresh numeric value newer than the last point extends the line; the window keeps the span of
        // the loaded series (at most 720 points). Stale, unavailable and older values never append.
        function tail(e) {
            if (!loaded || !e || hostWord(e) || e.stale) return;
            const v = numberOf(e.value), t = Date.parse(e.attributes && e.attributes.updated_at), last = hist[hist.length - 1];
            if (v === null || !Number.isFinite(t) || (last && t <= last[0])) return;
            const span = hist.length > 1 ? last[0] - hist[0][0] : 0;
            hist.push([t, v]);
            if (span > 0) while (hist.length > 2 && hist[0][0] < t - span) hist.shift();
            if (hist.length > 720) hist.splice(0, hist.length - 720);
        }
        // Host duty: resize(size) must carry px: {width, height} (the lab passes it; M2 hosts must too). A resize without px is ignored, and create gets
        // no size, so the spark keeps its default box until the first resize with px.
        function sparkState() {
            const e = lastView && lastView.entities.primary, pts = hostWord(e) ? [] : hist.map(p => p[1]), ext = extent(hist), s = { points: pts };
            if (dims) { s.w = dims.w; s.h = dims.h; }
            const now = e ? numberOf(e.value) : null, unit = e && e.unit ? e.unit : '', sd = decimalsOf(api, undefined);
            if (ext && pts.length) s.summary = api.t('homedash.sensor.summary', { min: F.valueText(ext.min, { decimals: sd, lang: api.lang, unit }), max: F.valueText(ext.max, { decimals: sd, lang: api.lang, unit }), now: F.valueText(now, { decimals: sd, lang: api.lang, unit }) });
            return s;
        }
        function render() {
            const e = lastView && lastView.entities.primary, v = e && !hostWord(e) ? numberOf(e.value) : null;
            api.setTone(e && e.tone ? e.tone : null);
            prim.set({ value: v, unit: (e && e.unit) || '', status: e ? e.status : 'unavailable', stale: !!(e && e.stale), readonly: !!(e && e.readonly) });
            if (sr) sr.textContent = readoutText(api, cfg, (e && e.unit) || '', v, null);
            if (lastShown !== undefined && lastShown !== v) flip(prim.el);
            lastShown = v;
            spark.set(sparkState());
            api.setStatus(statusText(api, e, null));
        }
        history(api, s => { hist = Array.isArray(s.points) ? s.points.filter(Array.isArray) : []; loaded = true; if (lastView) render(); });
        return {
            update(view) { lastView = view; tail(view.entities.primary); render(); },
            resize(size) {
                const px = size && size.px; if (!px || !Number.isFinite(px.width) || !Number.isFinite(px.height)) return;
                const w = px.width - 2, h = Math.round(px.height * 0.38);
                // Tactile draws into an inset window: 8 px in from the sides, and at 2x1 (aspect >= 1.6, as the family's container query) 4 px above the bottom with 3 px less height
                // so it clears the state text. The module owns these numbers and draws for the exact window; the CSS only positions the svg (inset), it never resizes it.
                const wide = px.width / px.height >= 1.6;
                dims = { w: Math.max(1, tact ? w - 16 : w), h: Math.max(1, tact ? h - (wide ? 11 : 8) : h) };
                if (lastView) spark.set(sparkState());
            },
            destroy() { prim.destroy(); spark.destroy(); if (sr) sr.remove(); }
        };
    }

    E.widgets.register('gauge', {
        bindings: ['primary'],
        variants: ['arc', 'needle', 'bar', 'tank'],
        create(api) {
            if (api.variant === 'needle' || api.variant === 'arc') return gauge(api, api.variant);
            return { update() { api.setStatus(null); } };
        }
    });
    E.widgets.register('temperature', {
        bindings: ['primary'],
        variants: ['numeral', 'thermometer', 'comfort'],
        create(api) {
            if (api.variant === 'numeral') return tread(api, true);
            return { update() { api.setStatus(null); } };
        }
    });
    E.widgets.register('sensor', {
        bindings: ['primary'],
        variants: ['numeral', 'sparkline', 'icon'],
        create(api) {
            if (api.variant === 'numeral') return tread(api, false);
            if (api.variant === 'sparkline') return sparkline(api);
            return { update() { api.setStatus(null); } };
        }
    });
})();
