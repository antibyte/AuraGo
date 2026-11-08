(function () {
    'use strict';
    // Pure math and small SVG helpers for the Synth Studio read-outs: the master
    // dB label, the LED level meter, the EQ and filter response curves and the
    // envelope shape. Everything here is feedback only; nothing touches audio.
    const FS = 48000;
    const F_MIN = 20, F_MAX = 20000;
    const clamp = (value, low, high) => Math.min(high, Math.max(low, value));
    const num = (value, fallback) => (Number.isFinite(Number(value)) ? Number(value) : fallback);

    // ---- master level: dB label and LED segments ---------------------------
    function gainToDb(gain) {
        const g = num(gain, 0);
        return g > 0.00001 ? 20 * Math.log10(g) : -Infinity;
    }
    function formatDb(gain) {
        const db = gainToDb(gain);
        if (!Number.isFinite(db)) return '-inf';
        const text = Math.abs(db) < 0.05 ? '0.0' : db.toFixed(1);
        return (db >= 0.05 ? '+' : '') + text + ' dB';
    }
    // Number of lit segments for a linear peak: `segments` steps over floorDb..0 dB.
    function meterLit(peak, segments = 16, floorDb = -48) {
        const db = gainToDb(peak);
        if (!Number.isFinite(db) || db <= floorDb) return 0;
        return clamp(Math.ceil(((db - floorDb) / -floorDb) * segments), 0, segments);
    }
    // Rises at once, falls a little per frame (a calm ballistic).
    function meterFollow(previous, target, fall = 0.35) {
        return target >= previous ? target : Math.max(target, previous - fall);
    }
    function segmentTone(index, count = 16) {
        const f = index / count;
        return f < 0.62 ? 'g' : f < 0.85 ? 'a' : 'r';
    }
    function meterMarkup(label, segments = 16) {
        let cells = '';
        for (let i = 0; i < segments; i++) cells += '<i class="ss-led-seg" data-tone="' + segmentTone(i, segments) + '"></i>';
        return '<div class="ss-led-meter" data-led-meter data-lit="0" role="img" aria-label="' + label + '">' + cells + '</div>';
    }
    function paintMeter(meter, lit) {
        if (!meter || meter.dataset.lit === String(lit)) return false;
        meter.dataset.lit = String(lit);
        const cells = meter.children;
        for (let i = 0; i < cells.length; i++) cells[i].classList.toggle('is-on', i < lit);
        return true;
    }

    // ---- biquad magnitude (Web Audio BiquadFilterNode formulas) -----------
    function biquad(type, frequency, gainDb, q) {
        const w0 = 2 * Math.PI * clamp(frequency, 10, FS / 2 - 1) / FS, cos = Math.cos(w0), sin = Math.sin(w0);
        const A = Math.pow(10, num(gainDb, 0) / 40), Q = Math.max(0.0001, num(q, 1));
        const aQ = sin / (2 * Q), aQdB = sin / (2 * Math.pow(10, Q / 20)), aS = sin / 2 * Math.SQRT2;
        const sA = 2 * Math.sqrt(A);
        let b, a;
        switch (type) {
            case 'lowpass': b = [(1 - cos) / 2, 1 - cos, (1 - cos) / 2]; a = [1 + aQdB, -2 * cos, 1 - aQdB]; break;
            case 'highpass': b = [(1 + cos) / 2, -(1 + cos), (1 + cos) / 2]; a = [1 + aQdB, -2 * cos, 1 - aQdB]; break;
            case 'bandpass': b = [aQ, 0, -aQ]; a = [1 + aQ, -2 * cos, 1 - aQ]; break;
            case 'notch': b = [1, -2 * cos, 1]; a = [1 + aQ, -2 * cos, 1 - aQ]; break;
            case 'peaking': b = [1 + aQ * A, -2 * cos, 1 - aQ * A]; a = [1 + aQ / A, -2 * cos, 1 - aQ / A]; break;
            case 'lowshelf':
                b = [A * ((A + 1) - (A - 1) * cos + sA * aS), 2 * A * ((A - 1) - (A + 1) * cos), A * ((A + 1) - (A - 1) * cos - sA * aS)];
                a = [(A + 1) + (A - 1) * cos + sA * aS, -2 * ((A - 1) + (A + 1) * cos), (A + 1) + (A - 1) * cos - sA * aS];
                break;
            case 'highshelf':
                b = [A * ((A + 1) + (A - 1) * cos + sA * aS), -2 * A * ((A - 1) + (A + 1) * cos), A * ((A + 1) + (A - 1) * cos - sA * aS)];
                a = [(A + 1) - (A - 1) * cos + sA * aS, 2 * ((A - 1) - (A + 1) * cos), (A + 1) - (A - 1) * cos - sA * aS];
                break;
            default: b = [1, 0, 0]; a = [1, 0, 0];
        }
        return { b, a };
    }
    function magnitudeDb(coefficients, frequency) {
        const w = 2 * Math.PI * frequency / FS, { b, a } = coefficients;
        const re = (c) => c[0] + c[1] * Math.cos(w) + c[2] * Math.cos(2 * w);
        const im = (c) => -c[1] * Math.sin(w) - c[2] * Math.sin(2 * w);
        const nr = re(b), ni = im(b), dr = re(a), di = im(a);
        const mag = Math.sqrt((nr * nr + ni * ni) / Math.max(1e-30, dr * dr + di * di));
        return 20 * Math.log10(Math.max(1e-6, mag));
    }
    function frequencies(points = 96) {
        const out = [];
        for (let i = 0; i < points; i++) out.push(F_MIN * Math.pow(F_MAX / F_MIN, i / (points - 1)));
        return out;
    }
    // The master EQ, exactly as createMaster() builds it (synth-studio-fx.js).
    const EQ_BANDS = [['lowshelf', 110, 0], ['peaking', 1000, 0.9], ['highshelf', 5000, 0]];
    function eqResponse(low, mid, high, points = 96) {
        const gains = [num(low, 0), num(mid, 0), num(high, 0)].map(g => clamp(g, -18, 18));
        const stages = EQ_BANDS.map(([type, f, q], i) => biquad(type, f, gains[i], q));
        return frequencies(points).map(f => stages.reduce((sum, s) => sum + magnitudeDb(s, f), 0));
    }
    // The designer cutoff knob maps to Hz as in createVoice() (synth-studio-voices.js),
    // without the per note floor of pitch*1.35. Web Audio low/high pass Q is in dB.
    function cutoffHz(cutoff) {
        return clamp(180 + Math.pow(clamp(num(cutoff, 0.65), 0.05, 1), 1.7) * 9500, 120, 16000);
    }
    const FILTER_TYPES = ['lowpass', 'highpass', 'bandpass', 'notch'];
    function filterResponse(type, cutoff, q, points = 96) {
        const kind = FILTER_TYPES.includes(type) ? type : 'lowpass';
        const stage = biquad(kind, cutoffHz(cutoff), 0, clamp(num(q, 1), 0.7, 15));
        return frequencies(points).map(f => magnitudeDb(stage, f));
    }

    // ---- SVG geometry -------------------------------------------------------
    const VIEW_W = 240, VIEW_H = 90;
    const freqX = (f, width = VIEW_W) => width * Math.log(clamp(f, F_MIN, F_MAX) / F_MIN) / Math.log(F_MAX / F_MIN);
    const dbY = (db, minDb, maxDb, height = VIEW_H) => height * (1 - (clamp(db, minDb, maxDb) - minDb) / (maxDb - minDb));
    function curvePaths(values, minDb, maxDb, width = VIEW_W, height = VIEW_H) {
        const n = values.length;
        const pts = values.map((db, i) => (width * i / (n - 1)).toFixed(1) + ' ' + dbY(db, minDb, maxDb, height).toFixed(1));
        const line = 'M' + pts.join('L');
        const base = dbY(0, minDb, maxDb, height).toFixed(1);
        return { line, fill: line + 'L' + width + ' ' + base + 'L0 ' + base + 'Z' };
    }
    function gridMarkup(minDb, maxDb) {
        let g = '';
        for (const f of [100, 1000, 10000]) g += '<line x1="' + freqX(f).toFixed(1) + '" x2="' + freqX(f).toFixed(1) + '" y1="0" y2="' + VIEW_H + '"/>';
        g += '<line class="ss-curve-zero" x1="0" x2="' + VIEW_W + '" y1="' + dbY(0, minDb, maxDb).toFixed(1) + '" y2="' + dbY(0, minDb, maxDb).toFixed(1) + '"/>';
        return '<g class="ss-curve-grid">' + g + '</g>';
    }
    const EQ_RANGE = [-20, 20], FILTER_RANGE = [-30, 20];
    function eqPaths(low, mid, high) {
        return curvePaths(eqResponse(low, mid, high), EQ_RANGE[0], EQ_RANGE[1]);
    }
    function filterPaths(type, cutoff, q) {
        return curvePaths(filterResponse(type, cutoff, q), FILTER_RANGE[0], FILTER_RANGE[1]);
    }
    function svgShell(kind, label, grid, extra) {
        return '<svg class="ss-curve ss-curve-' + kind + '" data-curve="' + kind + '" viewBox="0 0 ' + VIEW_W + ' ' + VIEW_H + '" preserveAspectRatio="none" role="img" aria-label="' + label + '">' + grid + '<path class="ss-curve-fill" d=""/><path class="ss-curve-line" d=""/>' + (extra || '') + '</svg>';
    }
    function eqMarkup(label) { return svgShell('eq', label, gridMarkup(EQ_RANGE[0], EQ_RANGE[1])); }
    function filterMarkup(label) {
        return svgShell('filter', label, gridMarkup(FILTER_RANGE[0], FILTER_RANGE[1]), '<line class="ss-curve-marker" x1="0" x2="0" y1="0" y2="' + VIEW_H + '"/>');
    }

    // ---- envelope shape -----------------------------------------------------
    // Segment widths grow with the square root of their time so a 4 s attack does
    // not squash a 20 ms one; the sustain stage is a fixed share of the width.
    function adsrPoints(attack, decay, sustain, release, width = VIEW_W, height = VIEW_H) {
        const a = clamp(num(attack, 0), 0, 30), d = clamp(num(decay, 0), 0, 30), r = clamp(num(release, 0), 0, 30);
        const s = clamp(num(sustain, 0), 0, 1.5);
        const weight = [Math.sqrt(a) + 0.12, Math.sqrt(d) + 0.12, 1.1, Math.sqrt(r) + 0.12];
        const total = weight.reduce((x, y) => x + y, 0);
        const top = Math.max(1, s), pad = 4, usable = height - pad * 2;
        const y = level => pad + usable * (1 - level / top);
        let x = 0;
        const xs = weight.map(w => (x += w / total * width));
        return [[0, y(0)], [xs[0], y(1)], [xs[1], y(s)], [xs[2], y(s)], [xs[3], y(0)]];
    }
    function adsrPaths(attack, decay, sustain, release) {
        const pts = adsrPoints(attack, decay, sustain, release);
        const line = 'M' + pts.map(p => p[0].toFixed(1) + ' ' + p[1].toFixed(1)).join('L');
        return { line, fill: line + 'L0 ' + pts[0][1].toFixed(1) + 'Z' };
    }
    function adsrMarkup(label) {
        return svgShell('adsr', label, '<g class="ss-curve-grid"><line class="ss-curve-zero" x1="0" x2="' + VIEW_W + '" y1="' + (VIEW_H - 4) + '" y2="' + (VIEW_H - 4) + '"/></g>');
    }

    // ---- painting -----------------------------------------------------------
    function setPaths(svg, paths) {
        const line = svg.querySelector('.ss-curve-line'), fill = svg.querySelector('.ss-curve-fill');
        if (line) line.setAttribute('d', paths.line);
        if (fill) fill.setAttribute('d', paths.fill);
    }
    function paintEq(svg, low, mid, high) { if (svg) setPaths(svg, eqPaths(low, mid, high)); }
    function paintFilter(svg, type, cutoff, q) {
        if (!svg) return;
        setPaths(svg, filterPaths(type, cutoff, q));
        const marker = svg.querySelector('.ss-curve-marker');
        if (marker) {
            const x = freqX(cutoffHz(cutoff)).toFixed(1);
            marker.setAttribute('x1', x);
            marker.setAttribute('x2', x);
        }
    }
    function paintAdsr(svg, attack, decay, sustain, release) { if (svg) setPaths(svg, adsrPaths(attack, decay, sustain, release)); }
    // Coalesces a burst of input events into one repaint per animation frame.
    function scheduler(callback) {
        let queued = false;
        return () => {
            if (queued) return;
            const raf = typeof requestAnimationFrame === 'function' ? requestAnimationFrame : fn => fn();
            queued = true;
            raf(() => { queued = false; callback(); });
        };
    }

    window.SynthStudioCurves = {
        FS, EQ_RANGE, FILTER_RANGE, VIEW_W, VIEW_H,
        gainToDb, formatDb, meterLit, meterFollow, segmentTone, meterMarkup, paintMeter,
        biquad, magnitudeDb, frequencies, eqResponse, cutoffHz, filterResponse, freqX, dbY,
        curvePaths, eqPaths, filterPaths, adsrPoints, adsrPaths, eqMarkup, filterMarkup, adsrMarkup,
        paintEq, paintFilter, paintAdsr, scheduler
    };
})();
