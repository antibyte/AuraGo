// HomeDash alert tone (decision D5; pure). The host computes a widget's tone and hands it to the widget as the entity view's `tone`;
// widgets only paint it.
// Rule (R7):
// Numeric: `options.warn` and `options.alert` are thresholds. Direction is "falling" only when both are numbers and `warn > alert`;
// otherwise "rising" (one threshold alone, or `warn <= alert`). Rising: value >= alert -> `crit`, else value >= warn -> `warn`.
// Falling: value <= alert -> `crit`, else value <= warn -> `warn`. Boundaries count as crossed; `min`/`max` never change the direction.
// Binary: a `binary_sensor` with device class `smoke`, `gas`, `moisture`, `problem`, `safety` or `carbon_monoxide` is `crit` while on.
// Unavailable or unknown entities, non-numeric values and values inside the thresholds have no tone (`null`).
// The host puts the tone on the entity view (`tone`); widgets paint it.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format) throw new Error('hd-tone.js: load hd-core.js and hd-format.js first');
    const SAFETY = Object.freeze(['smoke', 'gas', 'moisture', 'problem', 'safety', 'carbon_monoxide']);
    function num(v) {
        if (typeof v === 'number') return Number.isFinite(v) ? v : null;
        if (typeof v === 'string' && v.trim() !== '') { const n = Number(v); return Number.isFinite(n) ? n : null; }
        return null;
    }
    function direction(options) { const o = options || {}, w = num(o.warn), a = num(o.alert); return w !== null && a !== null && w > a ? 'falling' : 'rising'; }
    function numeric(value, options) {
        const o = options || {}, v = num(value), w = num(o.warn), a = num(o.alert);
        if (v === null || (w === null && a === null)) return null;
        if (direction(o) === 'falling') return a !== null && v <= a ? 'crit' : w !== null && v <= w ? 'warn' : null;
        return a !== null && v >= a ? 'crit' : w !== null && v >= w ? 'warn' : null;
    }
    function of(view, options) {
        if (!view || view.available === false || view.status === 'unavailable' || view.status === 'unknown') return null;
        if (view.kind === 'binary_sensor') return SAFETY.includes(view.device_class) && E.format.isOn(view) === true ? 'crit' : null;
        return numeric(view.value, options);
    }
    // Gauge zones for the same thresholds: ascending, clamped to [min, max], empty zones dropped.
    function zones(options) {
        const o = options || {}, w = num(o.warn), a = num(o.alert), lo0 = num(o.min) ?? 0, hi0 = num(o.max) ?? 100;
        const lo = Math.min(lo0, hi0), hi = Math.max(lo0, hi0), c = v => Math.min(hi, Math.max(lo, v)), out = [];
        if (direction(o) === 'falling') out.push({ from: lo, to: c(a), tone: 'crit' }, { from: c(a), to: c(w), tone: 'warn' });
        else { if (w !== null) out.push({ from: c(w), to: a !== null ? c(a) : hi, tone: 'warn' }); if (a !== null) out.push({ from: c(a), to: hi, tone: 'crit' }); }
        return out.filter(z => z.to > z.from);
    }
    E.tone = Object.freeze({ of, numeric, zones, direction, SAFETY });
})();
