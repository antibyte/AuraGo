// HomeDash gauge, temperature and sensor widgets. Heroes: gauge/needle (H5), gauge/arc (H6, Tactile substitutes the needle gauge),
// temperature/numeral (H7), sensor/sparkline (H8) and sensor/numeral (the H7 builder without degree and trend). G13 adds the level displays gauge/bar and gauge/tank,
// temperature/thermometer (glass tube with 24 h min/max marks) and temperature/comfort (temperature and humidity on a comfort zone), and sensor/icon (icon and value).
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
                // One row (a 2x1 under 96 px high, the 67 px rows of a 960 px page): the numeral takes the left part, so the chart takes the right 45 % and half the height (widgets.css positions it at 55 %).
                const wide = px.width / px.height >= 1.6, row = wide && px.height <= 95, w = row ? Math.round((px.width - 2) * 0.45) : px.width - 2, h = Math.round(px.height * (row ? 0.5 : 0.38));
                // Tactile draws into an inset window: 8 px in from the sides (8 px from the right edge only in a row), and at 2x1 (aspect >= 1.6, as the family's container query) 4 px above the bottom with 3 px less height
                // so it clears the state text. The module owns these numbers and draws for the exact window; the CSS only positions the svg (inset), it never resizes it.
                dims = { w: Math.max(1, tact ? w - (row ? 8 : 16) : w), h: Math.max(1, tact ? h - (wide ? 11 : 8) : h) };
                if (lastView) spark.set(sparkState());
            },
            destroy() { prim.destroy(); spark.destroy(); if (sr) sr.remove(); }
        };
    }

    // ---- G13 variants: gauge/bar, gauge/tank, temperature/thermometer, temperature/comfort, sensor/icon ----
    const finiteN = v => typeof v === 'number' && Number.isFinite(v), unit01 = (v, lo, hi) => Math.min(1, Math.max(0, (v - lo) / (hi - lo)));
    // Zones: options.zones when given, else the catalog options warn and alert: the warning zone runs from warn to the alert value, the alert zone to max.
    function zonesFor(api, max) {
        const o = api.widget.options || {}, own = zonesOf(api);
        if (own.length) return own;
        const warn = finiteN(o.warn) ? o.warn : null, alert = finiteN(o.alert) ? o.alert : null, out = [];
        if (warn !== null) out.push({ from: warn, to: alert !== null && alert > warn ? alert : max, tone: 'warn' });
        if (alert !== null) out.push({ from: alert, to: max, tone: 'crit' });
        return out;
    }
    const flagsOf = e => ({ status: e ? e.status : 'unavailable', pending: !!(e && e.pending), stale: !!(e && e.stale), readonly: !!(e && e.readonly) });
    // gauge/bar (horizontal) and gauge/tank (vertical): the level of a numeric entity between options.min and options.max, with its readout.
    function level(api, vertical) {
        const o = api.widget.options || {}, min = finiteN(o.min) ? o.min : 0, max = finiteN(o.max) && o.max > min ? o.max : min + 100, zones = zonesFor(api, max);
        const prim = api.prim.bar({ label: api.widget.title, variant: vertical ? 'v' : 'h', shape: vertical ? 'tank' : null, min, max, unit: o.unit, decimals: decimalsOf(api, 0), zones, readout: true, chars: 5, lang: api.lang });
        api.body.appendChild(prim.el);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        const toneAt = v => { const z = v === null ? [] : zones.filter(x => v >= x.from && v <= x.to); return (z.find(x => x.tone === 'crit') || z[0] || {}).tone || null; };
        return {
            update(view) {
                const e = view.entities.primary, v = e ? numberOf(e.value) : null, shown = hostWord(e) ? null : v;
                api.setTone(e && e.tone ? e.tone : toneAt(shown));
                prim.set(Object.assign({ value: shown, unit: (e && e.unit) || o.unit, tone: e && e.tone ? e.tone : null }, flagsOf(e)));
                api.setStatus(statusText(api, e, null));
            },
            destroy() { prim.destroy(); }
        };
    }

    // temperature/thermometer: a glass tube with a bulb, -10..40 C (14..104 F), the reading on top and the 24 h minimum and maximum as marks (options.show_minmax).
    function thermometer(api) {
        const o = api.widget.options || {}, dec = Number.isInteger(o.decimals) && o.decimals >= 0 && o.decimals <= 2 ? o.decimals : 1, marks = o.show_minmax !== false;
        const prim = api.prim.bar({ label: api.widget.title, variant: 'v', shape: 'thermo', min: -10, max: 40, unit: 'C', degree: true, decimals: dec, readout: true, chars: 4, lang: api.lang });
        const lo = el('span', { class: 'hd-bar-mark', 'data-end': 'lo', 'aria-hidden': 'true', hidden: '' }), hi = el('span', { class: 'hd-bar-mark', 'data-end': 'hi', 'aria-hidden': 'true', hidden: '' });
        if (marks) { prim.track.appendChild(lo); prim.track.appendChild(hi); }
        api.body.appendChild(prim.el);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        let lastView = null, range = null;
        function render() {
            const e = lastView && lastView.entities.primary, v = e && !hostWord(e) ? numberOf(e.value) : null, raw = e && typeof e.unit === 'string' ? e.unit : '', fahrenheit = /F/i.test(raw);
            const unit = (raw.replace(DEG, '') || 'C'), a = fahrenheit ? 14 : -10, b = fahrenheit ? 104 : 40;
            api.setTone(e && e.tone ? e.tone : null);
            prim.set(Object.assign({ value: v, min: a, max: b, unit, degree: true }, flagsOf(e)));
            const known = marks && range && v !== null;
            for (const [node, val] of [[lo, range && range.min], [hi, range && range.max]]) { setAttr(node, 'hidden', known ? null : ''); if (known) E.core.setVars(node, { '--hd-at': unit01(val, a, b).toFixed(4) }); }
            const text = F.valueText(v, { decimals: dec, lang: api.lang, unit: DEG + unit }), rangeText = range ? api.t('homedash.temperature.range', { min: F.valueText(range.min, { decimals: 1, lang: api.lang }), max: F.valueText(range.max, { decimals: 1, lang: api.lang, unit: DEG + unit }) }) : null;
            setAttr(prim.el, 'aria-valuetext', text);
            api.setStatus(statusText(api, e, rangeText));
        }
        if (marks) history(api, s => { range = extent(s.points); if (lastView) render(); });
        return { update(view) { lastView = view; render(); }, destroy() { prim.destroy(); } };
    }

    // temperature/comfort: the reading and the humidity, and a plot of both with the comfort zone (20-26 C, 40-60 %; 68-79 F). The plot spans 12-32 C (54-90 F) and 20-80 % so the zone is a third of
    // each axis; a reading beyond an edge sits on the edge. Without a humidity sensor the plot is the temperature band alone.
    function comfort(api) {
        const o = api.widget.options || {}, dec = Number.isInteger(o.decimals) && o.decimals >= 0 && o.decimals <= 2 ? o.decimals : 1;
        const plot = el('div', { class: 'hd-cf-plot', 'aria-hidden': 'true' }, [el('span', { class: 'hd-cf-zone' }), el('span', { class: 'hd-cf-dot' })]);
        const temp = api.prim.numeral({ chars: 4, decimals: dec, unit: 'C', degree: true, lang: api.lang }), hum = api.prim.numeral({ chars: 3, decimals: 0, unit: '%', lang: api.lang });
        const sr = el('span', { class: 'hd-sr-only' }), node = el('div', { class: 'hd-comfort' }, [plot, el('div', { class: 'hd-cf-read' }, [temp.el, hum.el]), sr]);
        setAttr(temp.el, 'aria-hidden', 'true'); setAttr(hum.el, 'aria-hidden', 'true');
        api.body.appendChild(node);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        return {
            update(view) {
                const e = view.entities.primary, h = view.entities.humidity || null, v = e && !hostWord(e) ? numberOf(e.value) : null, hv = h && !hostWord(h) ? numberOf(h.value) : null;
                const raw = e && typeof e.unit === 'string' ? e.unit : '', fahrenheit = /F/i.test(raw), unit = raw.replace(DEG, '') || 'C';
                const a = fahrenheit ? 54 : 12, b = fahrenheit ? 90 : 32, z0 = fahrenheit ? 68 : 20, z1 = fahrenheit ? 79 : 26;
                api.setTone(e && e.tone ? e.tone : null);
                temp.set(Object.assign({ value: v, unit }, flagsOf(e))); hum.set(Object.assign({ value: hv, unit: '%' }, flagsOf(h)));
                setAttr(node, 'data-hd-nohum', hv === null ? 'true' : null);
                setAttr(node, 'data-hd-empty', v === null ? 'true' : null);
                const zone = v === null ? null : v >= z0 && v <= z1 && (hv === null || (hv >= 40 && hv <= 60)) ? 'in' : 'out';
                setAttr(node, 'data-hd-zone', zone);
                E.core.setVars(plot, { '--hd-zx0': unit01(z0, a, b).toFixed(4), '--hd-zx1': unit01(z1, a, b).toFixed(4), '--hd-zy0': unit01(40, 20, 80).toFixed(4), '--hd-zy1': unit01(60, 20, 80).toFixed(4),
                    '--hd-cx': (v === null ? 0 : unit01(v, a, b)).toFixed(4), '--hd-cy': (hv === null ? 0.5 : unit01(hv, 20, 80)).toFixed(4) });
                sr.textContent = [F.valueText(v, { decimals: dec, lang: api.lang, unit: DEG + unit }), hv === null ? '' : F.valueText(hv, { decimals: 0, lang: api.lang, unit: '%' }),
                    zone === null ? '' : api.t(zone === 'in' ? 'homedash.comfort.inside' : 'homedash.comfort.outside')].filter(Boolean).join(', ');
                api.setStatus(statusText(api, e, null));
            },
            destroy() { temp.destroy(); hum.destroy(); node.remove(); }
        };
    }

    // sensor/icon: the entity's icon in a badge and its value; the icon takes the size of the card.
    function iconValue(api) {
        const cfg = { chars: 5, decimals: decimalsOf(api, 0), degree: false, trend: false }, { prim, inst } = readoutOf(api, cfg), badge = api.prim.iconBadge({ icon: 'gauge', size: 'l' });
        const sr = inst ? null : el('span', { class: 'hd-sr-only' });
        if (sr) setAttr(prim.el, 'aria-hidden', 'true');
        const node = el('div', { class: 'hd-sicon' }, sr ? [badge.el, prim.el, sr] : [badge.el, prim.el]);
        api.body.appendChild(node);
        setAttr(api.body, 'data-hd-nodata', api.t('homedash.state.no_data'));
        return {
            update(view) {
                const e = view.entities.primary, v = e && !hostWord(e) ? numberOf(e.value) : null, unit = (e && e.unit) || '';
                api.setTone(e && e.tone ? e.tone : null);
                badge.set(Object.assign({ icon: E.widgets.iconFor(api.widget, e) || 'gauge' }, flagsOf(e)));
                prim.set(Object.assign({ value: v, unit }, flagsOf(e)));
                if (sr) sr.textContent = readoutText(api, cfg, unit, v, null);
                api.setStatus(statusText(api, e, null));
            },
            destroy() { prim.destroy(); badge.destroy(); node.remove(); }
        };
    }

    E.widgets.register('gauge', {
        bindings: ['primary'],
        variants: ['arc', 'needle', 'bar', 'tank'],
        create(api) {
            if (api.variant === 'needle' || api.variant === 'arc') return gauge(api, api.variant);
            return level(api, api.variant === 'tank');
        }
    });
    E.widgets.register('temperature', {
        bindings: ['primary', 'humidity'],
        variants: ['numeral', 'thermometer', 'comfort'],
        create(api) {
            if (api.variant === 'numeral') return tread(api, true);
            return api.variant === 'thermometer' ? thermometer(api) : comfort(api);
        }
    });
    E.widgets.register('sensor', {
        bindings: ['primary'],
        variants: ['numeral', 'sparkline', 'icon'],
        create(api) {
            if (api.variant === 'numeral') return tread(api, false);
            if (api.variant === 'sparkline') return sparkline(api);
            return iconValue(api);
        }
    });
})();
