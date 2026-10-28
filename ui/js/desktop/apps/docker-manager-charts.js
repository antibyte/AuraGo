/* Docker Manager charts: pure functions that return SVG or HTML strings for the live numbers (sparklines, gauges, byte and
   percent text). A classic script (window.DockerManagerCharts) with no DOM work at load, so that
   scripts/test-docker-manager-ui.mjs can run it in a Node vm. Nothing here knows a colour: the shapes carry classes and
   data-level attributes and docker-manager's stylesheet paints them from theme tokens. Unknown values (-1 from the
   server, NaN, null) are gaps in a sparkline and an en dash in text. */
(function () {
    'use strict';

    const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'];
    const DASH = '–';
    const known = value => typeof value === 'number' && Number.isFinite(value) && value >= 0;
    // null, undefined and blank strings are unknown, not zero (Number(null) is 0).
    const num = value => (typeof value === 'number' ? value : typeof value === 'string' && value.trim() !== '' ? Number(value) : NaN);
    const round = value => Math.round(value * 100) / 100;
    const attr = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    const fixed = (value, digits, lang) => {
        try { return value.toLocaleString(lang || undefined, { minimumFractionDigits: digits, maximumFractionDigits: digits }); } catch (error) { return value.toFixed(digits); }
    };

    // 1536 -> "1.5 KB"; 2147483648 -> "2.0 GB"; unknown or negative -> en dash. Two significant digits from 10 upwards.
    function bytes(n, lang) {
        let value = num(n);
        if (!known(value)) return DASH;
        let unit = 0;
        while (value >= 1024 && unit < UNITS.length - 1) { value /= 1024; unit += 1; }
        return (unit === 0 ? String(Math.round(value)) : fixed(value, value >= 10 ? 0 : 1, lang)) + ' ' + UNITS[unit];
    }

    // 0.03 -> "0%"; 3.2 -> "3.2%"; 42.4 -> "42%"; unknown (-1 from the server) -> en dash.
    function percent(n, lang) {
        const value = num(n);
        if (!known(value)) return DASH;
        if (value < 0.05) return '0%';
        return (value < 10 ? fixed(value, 1, lang) : fixed(Math.round(value), 0, lang)) + '%';
    }

    // One number per sample of a StatsPoint series ("cpu", "mem", ...); anything that is not a known number becomes null.
    const series = (points, key) => (Array.isArray(points) ? points : []).map(point => (point && known(point[key]) ? point[key] : null));

    // Bytes per second of a cumulative counter ("net_rx") between the last two points; 0 after a counter reset, null without two points.
    function rate(points, key) {
        const list = Array.isArray(points) ? points : [];
        if (list.length < 2) return null;
        const a = list[list.length - 2], b = list[list.length - 1];
        const seconds = (Number(b && b.t) - Number(a && a.t)) / 1000;
        if (!(seconds > 0) || !known(a[key]) || !known(b[key])) return null;
        return b[key] >= a[key] ? (b[key] - a[key]) / seconds : 0;
    }

    // An inline SVG polyline of `points` (numbers; null or negative = gap), right-aligned in a window of `window` slots (default:
    // as many as there are points) and scaled to `max` (default: the highest value). With a `label` the SVG is an image for
    // assistive technology; without it, it is decorative.
    function sparkline(points, o) {
        const opt = o || {}, width = Number(opt.width) || 80, height = Number(opt.height) || 20, margin = 1.5;
        const values = (Array.isArray(points) ? points : []).map(value => (known(value) ? value : null));
        const slots = Math.max(Number(opt.window) || values.length, values.length, 2), offset = slots - values.length;
        const present = values.filter(value => value !== null);
        let max = Number(opt.max);
        if (!(max > 0)) max = present.length ? Math.max.apply(null, present) : 0;
        if (!(max > 0)) max = 1;
        const x = index => round(margin + (offset + index) * (width - 2 * margin) / (slots - 1));
        const y = value => round(height - margin - Math.min(value, max) / max * (height - 2 * margin));
        const runs = [];
        let run = [];
        values.forEach((value, index) => {
            if (value === null) { if (run.length) runs.push(run); run = []; } else run.push([x(index), y(value)]);
        });
        if (run.length) runs.push(run);
        const base = round(height - margin);
        let line = '', area = '';
        for (const part of runs) {
            line += 'M' + part.map(point => point[0] + ' ' + point[1]).join('L') + (part.length === 1 ? 'L' + part[0][0] + ' ' + part[0][1] : '');
            if (part.length > 1) area += 'M' + part[0][0] + ' ' + base + 'L' + part.map(point => point[0] + ' ' + point[1]).join('L') + 'L' + part[part.length - 1][0] + ' ' + base + 'Z';
        }
        const label = String(opt.label || '');
        const head = '<svg class="dm-spark' + (runs.length ? '' : ' is-empty') + '" viewBox="0 0 ' + width + ' ' + height + '" width="' + width + '" height="' + height + '" preserveAspectRatio="none" focusable="false"'
            + (label ? ' role="img" aria-label="' + attr(label) + '"' : ' aria-hidden="true"') + '>';
        if (!runs.length) return head + '<path class="dm-spark-base" d="M' + margin + ' ' + base + 'H' + (width - margin) + '"/></svg>';
        return head + (area ? '<path class="dm-spark-area" d="' + area + '"/>' : '') + '<path class="dm-spark-line" d="' + line + '"/></svg>';
    }

    // A labelled meter. `o.unit` is "percent" (value 0..100) or "bytes"; `o.text` replaces the generated value text. Without a
    // positive `max` (the Docker disk gauge: Docker's own usage has no ceiling) there is no bar, only label and value.
    // data-level is ok, warn (75 %), bad (90 %), plain (no ceiling) or none (unknown value).
    function gauge(value, max, o) {
        const opt = o || {}, label = String(opt.label || ''), lang = opt.lang;
        const v = known(num(value)) ? num(value) : NaN, m = Number(max), ceiling = m > 0 && Number.isFinite(m);
        const ratio = Number.isFinite(v) && ceiling ? Math.min(1, v / m) : 0;
        const level = !Number.isFinite(v) ? 'none' : !ceiling ? 'plain' : ratio >= 0.9 ? 'bad' : ratio >= 0.75 ? 'warn' : 'ok';
        let text = opt.text != null ? String(opt.text) : '';
        if (!text) {
            if (!Number.isFinite(v)) text = DASH;
            else if (opt.unit === 'percent') text = percent(v, lang);
            else if (opt.unit === 'bytes') text = bytes(v, lang) + (ceiling ? ' / ' + bytes(m, lang) : '');
            else text = fixed(v, 0, lang);
        }
        const head = '<div class="dm-gauge-head"><span class="dm-gauge-label">' + attr(label) + '</span><span class="dm-gauge-value">' + attr(text) + '</span></div>';
        const bar = ceiling && Number.isFinite(v)
            ? '<svg class="dm-gauge-bar" viewBox="0 0 100 4" preserveAspectRatio="none" aria-hidden="true" focusable="false"><rect class="dm-gauge-track" width="100" height="4"/><rect class="dm-gauge-fill" width="' + round(ratio * 100) + '" height="4"/></svg>'
            : '';
        const a11y = bar
            ? ' role="meter" aria-label="' + attr(label) + '" aria-valuemin="0" aria-valuemax="' + round(m) + '" aria-valuenow="' + round(v) + '" aria-valuetext="' + attr(text) + '"'
            : ' role="group" aria-label="' + attr(label + ': ' + text) + '"';
        return '<div class="dm-gauge" data-level="' + level + '"' + a11y + '>' + head + bar + '</div>';
    }

    // The two lines of a container's load cell (CPU, memory) for a StatsPoint series: the latest value as text next to a sparkline.
    // The CPU line scales to at least 10 %, so an idle container shows a flat line instead of noise. The window is as wide as the points
    // seen so far, at least 10 slots and at most 60 (the ring), so a young series fills the sparkline instead of hugging its right edge.
    // '' without points.
    function loadCell(points, o) {
        const opt = o || {}, list = Array.isArray(points) ? points : [];
        const last = list[list.length - 1];
        if (!last) return '';
        const cpu = series(list, 'cpu'), mem = series(list, 'mem');
        const cpuMax = Math.max(10, Math.max.apply(null, cpu.filter(value => value !== null).concat([0])));
        const line = (text, spark) => '<span class="dm-load-line"><span class="dm-load-val">' + attr(text) + '</span>' + spark + '</span>';
        const slots = opt.window || Math.min(60, Math.max(list.length, 10));
        return line(percent(last.cpu, opt.lang), sparkline(cpu, { width: 56, height: 16, max: cpuMax, window: slots }))
            + line(bytes(last.mem, opt.lang), sparkline(mem, { width: 56, height: 16, window: slots }));
    }

    window.DockerManagerCharts = { bytes, percent, series, rate, sparkline, gauge, loadCell };
})();
