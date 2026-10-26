(function () {
    'use strict';

    // DOM-free scope math for the scopes panel (2f; spec §6.7, §8.5; plan index §4.26.2–§4.26.4). The panel
    // (video-studio-scopes.js) reads one point-sampled 256 × 144 copy of the finished frame (painter readScaled, index
    // §4.26.1) and draws what this module returns:
    // - analyse(frame): RGB and luma histograms, the waveform (luma per column) and the vectorscope (Cb/Cr) counts;
    // - histogramImage, waveformImage and vectorscopeImage: RGBA8 images with one pixel per bin, in fixed colours for
    //   both desktop themes and without text;
    // - graticule(kind): the lines, circles and squares the panel strokes over an image, in continuous plot pixels
    //   (pixel i covers [i, i + 1));
    // - due(last, now, playing): the readback throttle; prefs(raw): the stored panel preference.
    // Luma and colour difference are BT.709, display-referred and full range, on the 8-bit values as they are (no
    // linearisation). Node tests check the module against analytic values; there is no Go side.
    const SIZE = Object.freeze({ width: 256, height: 144 });
    const HISTOGRAM = Object.freeze({ width: 256, height: 100 });
    const WAVEFORM = Object.freeze({ width: 256, height: 128 });
    const VECTORSCOPE = Object.freeze({ width: 128, height: 128 });
    const READ_INTERVAL_MS = 100;
    const KINDS = Object.freeze(['histogram', 'waveform', 'vectorscope']);
    const rgb = (r, g, b) => Object.freeze([r, g, b]);
    // Image colours (RGB 0..255; every pixel has alpha 255) and graticule strokes (2D context colours, lineWidth 1).
    const COLORS = Object.freeze({
        BG: rgb(12, 15, 21), TRACE: rgb(140, 255, 166), HR: rgb(255, 80, 80), HG: rgb(80, 230, 110), HB: rgb(90, 140, 255), LUMA: rgb(235, 235, 235),
        GRID: 'rgba(255, 255, 255, 0.18)', SKIN: 'rgba(255, 190, 140, 0.75)', TARGET: 'rgba(255, 255, 255, 0.5)'
    });
    const KR = 0.2126, KG = 0.7152, KB = 0.0722;
    // 1.8556 = 2(1 − KB) and 1.5748 = 2(1 − KR), so Cb and Cr lie in −0.5..0.5.
    const CB_SCALE = 1.8556 * 255, CR_SCALE = 1.5748 * 255;
    // The NTSC I axis at 123° in the U/V plane, with U = 0.492111(B − Y) and V = 0.877283(R − Y).
    const I_AXIS = 123 * Math.PI / 180;
    // The vectorscope targets: the 75 % colour bars red, yellow, green, cyan, blue and magenta.
    const TARGETS = Object.freeze([[191, 0, 0], [191, 191, 0], [0, 191, 0], [0, 191, 191], [0, 0, 191], [191, 0, 191]]);
    const round = value => Math.floor(value + 0.5);
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
    const line = (x0, y0, x1, y1, color) => ({ kind: 'line', x0, y0, x1, y1, color });

    // Y = 0.2126 R + 0.7152 G + 0.0722 B, real-valued on 0..255.
    function luma(r, g, b) { return KR * r + KG * g + KB * b; }
    // {cb, cr}: Cb = (B − Y)/(1.8556·255) and Cr = (R − Y)/(1.5748·255). Blue gives Cb +0.5 and yellow −0.5; red gives
    // Cr +0.5 and cyan −0.5. B − Y and R − Y are written as weighted channel differences, which are exactly 0 for every
    // grey. analyse inlines the same formulas.
    function chroma(r, g, b) { return { cb: (KR * (b - r) + KG * (b - g)) / CB_SCALE, cr: (KG * (r - g) + KB * (r - b)) / CR_SCALE }; }
    // The skin-tone line: the I axis mapped into Cb/Cr, about 2.347370 rad (134.494°) counter-clockwise from +Cb.
    function skinAngle() { return Math.atan2(Math.sin(I_AXIS) / (0.877283 * 1.5748), Math.cos(I_AXIS) / (0.492111 * 1.8556)); }

    // The counts of one frame {width, height, data: top-down RGBA8, alpha ignored}, or null for an empty or short frame:
    // - histogram {r, g, b, y}: a Uint32Array(256) each; y counts Y8 = min(255, round(Y)), so a grey v counts in bin v;
    // - waveform: Uint32Array(256 × 128) at column floor(x·256/w) and row 127 − (Y8 >> 1) (row 0 holds Y8 254 and 255);
    // - vectorscope: Uint32Array(128 × 128) at column floor((Cb + 0.5)·128) and row floor((0.5 − Cr)·128), both clamped
    //   to 0..127 (Cr points up): red lands in (49, 0), grey in (64, 64), blue in (127, 69).
    function analyse(frame) {
        const w = frame ? Math.floor(Number(frame.width)) : 0, h = frame ? Math.floor(Number(frame.height)) : 0, data = frame && frame.data;
        if (!(w > 0) || !(h > 0) || !data || data.length < w * h * 4) return null;
        const r = new Uint32Array(256), g = new Uint32Array(256), b = new Uint32Array(256), y = new Uint32Array(256);
        const waveform = new Uint32Array(WAVEFORM.width * WAVEFORM.height), vectorscope = new Uint32Array(VECTORSCOPE.width * VECTORSCOPE.height);
        const columns = new Uint32Array(w), last = VECTORSCOPE.width - 1;
        for (let x = 0; x < w; x++) columns[x] = Math.floor(x * WAVEFORM.width / w);
        for (let row = 0, o = 0; row < h; row++) {
            for (let x = 0; x < w; x++, o += 4) {
                const R = data[o], G = data[o + 1], B = data[o + 2], Y8 = Math.min(255, round(luma(R, G, B)));
                r[R]++; g[G]++; b[B]++; y[Y8]++;
                waveform[(WAVEFORM.height - 1 - (Y8 >> 1)) * WAVEFORM.width + columns[x]]++;
                const cb = (KR * (B - R) + KG * (B - G)) / CB_SCALE, cr = (KG * (R - G) + KB * (R - B)) / CR_SCALE;
                vectorscope[clamp(Math.floor((0.5 - cr) * VECTORSCOPE.height), 0, last) * VECTORSCOPE.width + clamp(Math.floor((cb + 0.5) * VECTORSCOPE.width), 0, last)]++;
            }
        }
        return { samples: w * h, histogram: { r, g, b, y }, waveform, vectorscope };
    }

    // A width × height RGBA8 image filled with BG.
    function background(width, height) {
        const out = new Uint8ClampedArray(width * height * 4), [r, g, b] = COLORS.BG;
        for (let i = 0; i < out.length; i += 4) { out[i] = r; out[i + 1] = g; out[i + 2] = b; out[i + 3] = 255; }
        return out;
    }
    // The 256 × 100 histogram (index §4.26.4). Per bin k, from the bottom, the R, G and B bars each add half their colour
    // to BG where they reach (components rounded and capped at 255), and the luma line goes over them.
    // - The bars scale to the peak of bins 1..254, or of all bins when those are empty, so a black or white letterbox
    //   clips at 100 instead of flattening the rest.
    // - The luma line joins t(k) = min(99, 100 − HY(k)) of neighbouring bins.
    function histogramImage(analysis) {
        const W = HISTOGRAM.width, H = HISTOGRAM.height, out = background(W, H);
        if (!analysis) return out;
        const { r, g, b, y } = analysis.histogram, lists = [r, g, b, y];
        let peak = 0;
        lists.forEach(list => { for (let k = 1; k <= 254; k++) peak = Math.max(peak, list[k]); });
        if (!peak) lists.forEach(list => { for (let k = 0; k <= 255; k++) peak = Math.max(peak, list[k]); });
        if (!peak) return out;
        const height = n => (n ? clamp(round(H * n / peak), 1, H) : 0);
        const bars = [[r, COLORS.HR], [g, COLORS.HG], [b, COLORS.HB]];
        for (let k = 0; k < W; k++) {
            const tops = bars.map(([list]) => H - height(list[k]));
            for (let row = 0; row < H; row++) {
                let red = COLORS.BG[0], green = COLORS.BG[1], blue = COLORS.BG[2];
                for (let c = 0; c < 3; c++) if (row >= tops[c]) { const colour = bars[c][1]; red += colour[0] / 2; green += colour[1] / 2; blue += colour[2] / 2; }
                const o = (row * W + k) * 4;
                out[o] = Math.min(255, round(red)); out[o + 1] = Math.min(255, round(green)); out[o + 2] = Math.min(255, round(blue));
            }
        }
        for (let k = 0, previous = -1; k < W; k++) {
            const top = Math.min(H - 1, H - height(y[k])), from = previous < 0 ? top : previous;
            for (let row = Math.min(from, top); row <= Math.max(from, top); row++) out.set(COLORS.LUMA, (row * W + k) * 4);
            previous = top;
        }
        return out;
    }
    // A count image, one pixel per bin (index §4.26.4): v(n) = 0 for n = 0, else min(1, ln(1 + n)/ln(1 + K)), and the
    // pixel is round(BG + (TRACE − BG)·v). K, at least 1, is the count that saturates a bin.
    function countImage(counts, width, height, saturate) {
        const out = background(width, height), bg = COLORS.BG, trace = COLORS.TRACE, scale = Math.log(1 + Math.max(1, saturate));
        if (!counts) return out;
        for (let i = 0; i < width * height; i++) {
            const n = counts[i];
            if (!n) continue;
            const v = Math.min(1, Math.log(1 + n) / scale), o = i * 4;
            for (let c = 0; c < 3; c++) out[o + c] = round(bg[c] + (trace[c] - bg[c]) * v);
        }
        return out;
    }
    // The 256 × 128 waveform: K = samples/1024 (36 at 256 × 144).
    function waveformImage(analysis) { return countImage(analysis && analysis.waveform, WAVEFORM.width, WAVEFORM.height, analysis ? analysis.samples / 1024 : 1); }
    // The 128 × 128 vectorscope: K = samples/2304 (16 at 256 × 144).
    function vectorscopeImage(analysis) { return countImage(analysis && analysis.vectorscope, VECTORSCOPE.width, VECTORSCOPE.height, analysis ? analysis.samples / 2304 : 1); }

    // The strokes over one scope (index §4.26.4), a fresh list per call; [] for an unknown kind:
    // - histogram: the quarter lines x = 64, 128 and 192;
    // - waveform: the levels 0, 25, 50, 75 and 100 % (L 0, 64, 128, 191, 255) at y = 127.5 − (L >> 1);
    // - vectorscope: the axes through (64, 64), the rings |C| = 0.25 and 0.5, the skin-tone line to radius 64, and 5 × 5
    //   squares centred on the 75 % bars.
    function graticule(kind) {
        if (kind === 'histogram') return [64, 128, 192].map(x => line(x, 0, x, HISTOGRAM.height, COLORS.GRID));
        if (kind === 'waveform') return [0, 64, 128, 191, 255].map(level => line(0, 127.5 - (level >> 1), WAVEFORM.width, 127.5 - (level >> 1), COLORS.GRID));
        if (kind !== 'vectorscope') return [];
        const c = VECTORSCOPE.width / 2, theta = skinAngle();
        const shapes = [line(c, 0, c, VECTORSCOPE.height, COLORS.GRID), line(0, c, VECTORSCOPE.width, c, COLORS.GRID),
            { kind: 'circle', cx: c, cy: c, r: 32, color: COLORS.GRID }, { kind: 'circle', cx: c, cy: c, r: 64, color: COLORS.GRID },
            line(c, c, c + 64 * Math.cos(theta), c - 64 * Math.sin(theta), COLORS.SKIN)];
        TARGETS.forEach(([r, g, b]) => {
            const { cb, cr } = chroma(r, g, b);
            shapes.push({ kind: 'rect', x: (cb + 0.5) * VECTORSCOPE.width - 2.5, y: (0.5 - cr) * VECTORSCOPE.height - 2.5, w: 5, h: 5, color: COLORS.TARGET });
        });
        return shapes;
    }

    // The readback throttle (index §4.26.2): every paint while paused, at most every READ_INTERVAL_MS while playing.
    function due(last, now, playing) { return !playing || now - last >= READ_INTERVAL_MS; }

    // The stored panel preference {"open": bool, "show": [kinds]} (index §4.26.3), as {open, show} with show in KINDS
    // order. Invalid JSON gives {open: false, show: all three}. Unknown kinds are dropped, and an empty show means all
    // three.
    function prefs(raw) {
        let parsed = null;
        try { parsed = JSON.parse(String(raw)); } catch (_) { parsed = null; }
        const valid = !!parsed && typeof parsed === 'object';
        const listed = valid && Array.isArray(parsed.show) ? parsed.show : [];
        const show = KINDS.filter(kind => listed.includes(kind));
        return { open: valid && parsed.open === true, show: show.length ? show : KINDS.slice() };
    }

    window.VideoStudioScopeMath = { SIZE, HISTOGRAM, WAVEFORM, VECTORSCOPE, READ_INTERVAL_MS, KINDS, COLORS, luma, chroma, skinAngle, analyse, histogramImage, waveformImage, vectorscopeImage, graticule, due, prefs };
})();
