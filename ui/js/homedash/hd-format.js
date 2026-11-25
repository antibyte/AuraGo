// HomeDash formatting (pure): numbers and units via Intl in the document language, tabular split into int/frac/unit,
// relative time, durations and state words.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-format.js: load hd-core.js first');
    const DASH = String.fromCodePoint(0x2013), NBSP = String.fromCodePoint(0xa0), STATE_PREFIX = 'homedash.state.';
    const ON = new Set(['on', 'open', 'opening', 'unlocked', 'playing', 'home', 'true']);
    const OFF = new Set(['off', 'closed', 'closing', 'locked', 'idle', 'paused', 'standby', 'not_home', 'false']);
    const cache = new Map();
    const isNum = v => typeof v === 'number' && Number.isFinite(v);
    function nf(lang, d, grouping) {
        const key = (lang || 'en') + '|' + d + '|' + grouping;
        if (!cache.has(key)) cache.set(key, new Intl.NumberFormat(lang || 'en', { minimumFractionDigits: d, maximumFractionDigits: d, useGrouping: grouping, signDisplay: 'negative' }));
        return cache.get(key);
    }
    const decimalsOf = (v, o) => (o.decimals === undefined || o.decimals === null ? (Number.isInteger(v) ? 0 : Math.abs(v) < 100 ? 1 : 0) : o.decimals);
    function number(value, opts) { const o = opts || {}; return isNum(value) ? nf(o.lang, decimalsOf(value, o), o.grouping !== false).format(value) : ''; }
    function split(value, opts) {
        const o = opts || {}, unit = o.unit ? String(o.unit) : '';
        if (!isNum(value)) return { int: DASH, frac: '', unit, empty: true };
        let int = '', frac = '';
        for (const p of nf(o.lang, decimalsOf(value, o), o.grouping !== false).formatToParts(value)) { if (p.type === 'decimal' || p.type === 'fraction') frac += p.value; else int += p.value; }
        return { int, frac, unit, empty: false };
    }
    function valueText(value, opts) { const t = number(value, opts); if (!t) return DASH; const u = opts && opts.unit; return u ? t + NBSP + u : t; }
    const percent = (fr, opts) => (isNum(fr) ? new Intl.NumberFormat((opts && opts.lang) || 'en', { style: 'percent', maximumFractionDigits: 0, signDisplay: 'negative' }).format(fr) : DASH);
    function relTime(then, now, lang) {
        if (!isNum(then) || !isNum(now)) return '';
        const s = Math.round((then - now) / 1000), a = Math.abs(s), r = new Intl.RelativeTimeFormat(lang || 'en', { numeric: 'auto' });
        if (a < 60) return r.format(s, 'second'); if (a < 3600) return r.format(Math.round(s / 60), 'minute');
        if (a < 86400) return r.format(Math.round(s / 3600), 'hour'); return r.format(Math.round(s / 86400), 'day');
    }
    function duration(seconds, lang) {
        if (!isNum(seconds) || seconds < 0) return DASH;
        // Largest non-zero unit plus only the next smaller unit (omitted when zero), so 7215 s reads "2h", never "2h 15s".
        const units = [['day', 86400], ['hour', 3600], ['minute', 60], ['second', 1]], fmt = (unit, n) => new Intl.NumberFormat(lang || 'en', { style: 'unit', unit, unitDisplay: 'narrow' }).format(n);
        let rest = Math.floor(seconds);
        for (let i = 0; i < units.length; i++) {
            const n = Math.floor(rest / units[i][1]); if (n === 0) continue;
            rest -= n * units[i][1];
            const next = units[i + 1], m = next ? Math.floor(rest / next[1]) : 0;
            return m > 0 ? fmt(units[i][0], n) + ' ' + fmt(next[0], m) : fmt(units[i][0], n);
        }
        return fmt('second', 0);
    }
    function isOn(view) {
        if (!view) return null; const v = view.value;
        if (v === true || v === false) return v;
        const s = typeof v === 'string' ? v.toLowerCase() : null;
        return s && ON.has(s) ? true : s && OFF.has(s) ? false : null;
    }
    function stateWord(view) {
        if (!view || view.available === false || view.status === 'unavailable') return 'unavailable';
        if (view.status === 'unknown') return 'unknown'; if (view.status === 'error') return 'error'; if (view.pending) return 'pending';
        const on = isOn(view); return on === true ? 'on' : on === false ? 'off' : 'value';
    }
    const stateText = (view, t) => { const w = stateWord(view); return w === 'value' ? '' : t(STATE_PREFIX + w); };
    E.format = Object.freeze({ DASH, NBSP, STATE_PREFIX, number, split, valueText, percent, relTime, duration, isOn, stateWord, stateText });
})();
