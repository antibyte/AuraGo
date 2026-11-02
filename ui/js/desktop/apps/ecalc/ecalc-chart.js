/* ECalc chart: the line chart of a calculator that declares `chart` in the catalog (a curve of one quantity over another,
   for example the charge curve of an RC circuit). A DOM-free classic script that publishes window.ECalcChart and builds
   one inline SVG string, like the schematics do: every stroke is currentColor, the colours of the curve and the dot come
   from the classes ec-chart-* in css/desktop-app-ecalc.css (tokens only).

   ECalcChart.svg(spec, sample) -> SVG markup, or '' when there is nothing to draw (no finite sample, xMax not > 0).
     spec:   {xMax, samples, label, xLabel, yLabel, fmtX, fmtY, point}
             xMax      the curve runs from x = 0 to xMax (base units)
             samples   number of points, clamped to 11..401 (default 101)
             label     accessible name of the drawing
             xLabel, yLabel   axis titles; the text behind the first underscore is a subscript (V_C)
             fmtX, fmtY       tick text of a value (the page passes the units formatter, so ticks read "1.25 ms")
             point     {x, y} of the current operating point, drawn as a dot with guides, or null
     sample: x -> y (base units); a non-finite y leaves a gap in the curve.
   Both axes carry five ticks. The x axis is 0 .. xMax. The y axis always contains 0 and runs in steps of 1, 1.5, 2, 2.5, 3, 4 or 5
   times a power of ten, so the tick values are round numbers. */
(function () {
    'use strict';

    const W = 320, H = 200;
    const LEFT = 58, RIGHT = 12, TOP = 18, BOTTOM = 46;
    const TICKS = 5, MIN_SAMPLES = 11, MAX_SAMPLES = 401, DEFAULT_SAMPLES = 101;
    const escape = text => String(text).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
    const num = x => String(Math.round(x * 100) / 100);
    const round12 = x => Number(x.toPrecision(12));

    // V_C -> V<tspan>C</tspan>: the text behind the first underscore is the subscript.
    function titleMarkup(text) {
        const label = String(text == null ? '' : text), at = label.indexOf('_');
        if (at <= 0) return escape(label);
        return escape(label.slice(0, at)) + '<tspan class="ec-chart-sub" dy="3">' + escape(label.slice(at + 1)) + '</tspan>';
    }

    // The y axis for data between lo and hi: {min, max, step} with max = min + 4 * step, 0 inside.
    function axis(lo, hi) {
        const low = Math.min(0, lo), high = Math.max(0, hi);
        if (!(high > low)) return { min: 0, max: TICKS - 1, step: 1 };
        const raw = (high - low) / (TICKS - 1);
        for (let scale = Math.pow(10, Math.floor(Math.log10(raw))); ; scale *= 10) {
            for (const factor of [1, 1.5, 2, 2.5, 3, 4, 5]) {
                const step = factor * scale, min = round12(Math.floor(low / step + 1e-9) * step);
                if (min + step * (TICKS - 1) >= high - step * 1e-9) return { min, max: round12(min + step * (TICKS - 1)), step };
            }
        }
    }

    function svg(spec, sample) {
        const xMax = Number(spec && spec.xMax);
        if (!(xMax > 0) || !Number.isFinite(xMax) || typeof sample !== 'function') return '';
        const count = Math.max(MIN_SAMPLES, Math.min(MAX_SAMPLES, Math.round(Number(spec.samples) || DEFAULT_SAMPLES)));
        const points = [];
        let lo = Infinity, hi = -Infinity;
        for (let i = 0; i < count; i += 1) {
            const x = xMax * i / (count - 1);
            let y;
            try { y = sample(x); } catch (error) { y = NaN; }
            points.push([x, y]);
            if (Number.isFinite(y)) { lo = Math.min(lo, y); hi = Math.max(hi, y); }
        }
        if (!Number.isFinite(lo)) return '';
        const dot = spec.point && Number.isFinite(spec.point.x) && Number.isFinite(spec.point.y) && spec.point.x >= 0 && spec.point.x <= xMax ? spec.point : null;
        if (dot) { lo = Math.min(lo, dot.y); hi = Math.max(hi, dot.y); }
        const y = axis(lo, hi);
        const left = LEFT, right = W - RIGHT, top = TOP, bottom = H - BOTTOM;
        const px = x => left + x / xMax * (right - left);
        const py = value => bottom - (value - y.min) / (y.max - y.min) * (bottom - top);
        const fmtX = spec.fmtX || String, fmtY = spec.fmtY || String;

        let grid = '', labels = '';
        for (let i = 0; i < TICKS; i += 1) {
            const x = round12(xMax * i / (TICKS - 1)), value = round12(y.min + y.step * i);
            const gx = num(px(x)), gy = num(py(value));
            if (i > 0) grid += '<path class="ec-chart-grid" d="M' + gx + ' ' + top + 'V' + bottom + '"/>';
            if (i > 0) grid += '<path class="ec-chart-grid" d="M' + left + ' ' + gy + 'H' + right + '"/>';
            labels += '<text class="ec-chart-text" x="' + gx + '" y="' + (bottom + 13) + '" text-anchor="' + (i === TICKS - 1 ? 'end' : 'middle') + '" stroke="none" fill="currentColor">' + escape(fmtX(x)) + '</text>';
            labels += '<text class="ec-chart-text" x="' + (left - 6) + '" y="' + num(py(value) + 3.5) + '" text-anchor="end" stroke="none" fill="currentColor">' + escape(fmtY(value)) + '</text>';
        }
        labels += '<text class="ec-chart-text" x="' + num((left + right) / 2) + '" y="' + (H - 6) + '" text-anchor="middle" stroke="none" fill="currentColor">' + titleMarkup(spec.xLabel) + '</text>';
        labels += '<text class="ec-chart-text" x="' + left + '" y="' + (top - 7) + '" text-anchor="start" stroke="none" fill="currentColor">' + titleMarkup(spec.yLabel) + '</text>';

        let d = '', pen = false;
        for (const [x, value] of points) {
            if (!Number.isFinite(value)) { pen = false; continue; }
            d += (pen ? 'L' : 'M') + num(px(x)) + ' ' + num(py(value));
            pen = true;
        }
        let marker = '';
        if (dot) {
            const cx = num(px(dot.x)), cy = num(py(dot.y));
            marker = '<path class="ec-chart-guide" d="M' + cx + ' ' + bottom + 'V' + cy + 'H' + left + '"/><circle class="ec-chart-dot" cx="' + cx + '" cy="' + cy + '" r="4"/>';
        }
        return '<svg class="ec-chart-svg" viewBox="0 0 ' + W + ' ' + H + '" role="img" aria-label="' + escape(spec.label || '') + '" focusable="false" preserveAspectRatio="xMidYMid meet" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">'
            + grid
            + '<path class="ec-chart-axis" d="M' + left + ' ' + top + 'V' + bottom + 'H' + right + '"/>'
            + '<path class="ec-chart-curve" d="' + d + '"/>'
            + marker + labels + '</svg>';
    }

    window.ECalcChart = { svg };
})();
