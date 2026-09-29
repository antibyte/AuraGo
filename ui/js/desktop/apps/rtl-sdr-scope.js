(function () {
    'use strict';
    // Band scope of the RTL-SDR display: averaged spectrum with peak hold above a
    // waterfall whose history stays aligned to frequency while tuning.
    const BINS = 1024, ROWS = 360, ROW_HEIGHT = 2, RANGE_DB = 70;
    const STOPS = [[0, 3, 8, 14], [.17, 8, 30, 78], [.36, 13, 94, 168], [.52, 24, 178, 176], [.66, 104, 224, 124], [.8, 246, 214, 78], [.92, 255, 130, 62], [1, 255, 238, 228]];
    const PALETTE = new Uint8ClampedArray(256 * 3);
    for (let i = 0, k = 0; i < 256; i++) {
        const v = i / 255;
        while (k < STOPS.length - 2 && v > STOPS[k + 1][0]) k++;
        const a = STOPS[k], b = STOPS[k + 1], f = Math.max(0, Math.min(1, (v - a[0]) / (b[0] - a[0])));
        for (let c = 0; c < 3; c++) PALETTE[i * 3 + c] = a[c + 1] + (b[c + 1] - a[c + 1]) * f;
    }
    const TICKS = [5e3, 1e4, 2.5e4, 5e4, 1e5, 2e5, 2.5e5, 5e5, 1e6];
    const COLOR = { grid: 'rgba(134,243,223,.09)', axis: 'rgba(134,243,223,.55)', trace: '#86f3df', peak: 'rgba(255,180,84,.5)', marker: '#ffb454', band: 'rgba(134,243,223,.1)', edge: 'rgba(134,243,223,.35)', tip: 'rgba(4,10,13,.88)' };

    // Received history outlives the canvas, so switching views keeps the waterfall.
    function memory() {
        const history = document.createElement('canvas');
        history.width = BINS; history.height = ROWS;
        const paint = history.getContext('2d');
        return { history, paint, line: paint.createImageData(BINS, 1), rows: [], average: new Float32Array(BINS).fill(-120), peak: new Float32Array(BINS).fill(-120), head: 0, floor: -90, view: null };
    }

    function create(canvas, options) {
        const c = canvas.getContext('2d'), m = options.memory || memory();
        let tuned = null, hover = null, frame = 0, width = 0, height = 0, ratio = 1;

        function resize() {
            const box = canvas.getBoundingClientRect(), w = Math.max(120, Math.round(box.width)), hgt = Math.max(60, Math.round(box.height)), r = Math.min(2, window.devicePixelRatio || 1);
            if (w === width && hgt === height && r === ratio) return;
            width = w; height = hgt; ratio = r;
            canvas.width = Math.round(w * r); canvas.height = Math.round(hgt * r);
        }

        function push(receiver, tuning) {
            tuned = tuning;
            const bins = receiver.spectrum;
            if (!bins || !bins.length) { schedule(); return; }
            const next = { center: receiver.center_hz || tuning.frequency_hz, span: receiver.span_hz || 2400000 };
            const moved = !m.view || m.view.span !== next.span || Math.abs(m.view.center - next.center) > next.span / BINS;
            m.view = next;
            const sample = [];
            for (let i = 0; i < BINS; i++) {
                const db = Number(bins[Math.min(bins.length - 1, Math.floor(i / BINS * bins.length))]) || -120;
                m.average[i] = moved ? db : m.average[i] * .55 + db * .45;
                m.peak[i] = moved ? db : Math.max(m.peak[i] - .6, db);
                if (i % 16 === 0) sample.push(db);
            }
            sample.sort((a, b) => a - b);
            const median = sample[sample.length >> 1];
            m.floor = m.rows.length ? m.floor * .9 + median * .1 : median;
            for (let i = 0; i < BINS; i++) {
                const p = Math.max(0, Math.min(255, Math.round((m.average[i] - m.floor + 9) / RANGE_DB * 255))) * 3, o = i * 4;
                m.line.data[o] = PALETTE[p]; m.line.data[o + 1] = PALETTE[p + 1]; m.line.data[o + 2] = PALETTE[p + 2]; m.line.data[o + 3] = 255;
            }
            m.paint.putImageData(m.line, 0, m.head);
            m.rows.unshift({ y: m.head, center: next.center, span: next.span });
            if (m.rows.length > ROWS - 1) m.rows.pop();
            m.head = (m.head + 1) % ROWS;
            schedule();
        }

        const left = () => m.view.center - m.view.span / 2;
        const x = hz => (hz - left()) / m.view.span * width;
        const frequencyAt = clientX => { if (!m.view) return null; const box = canvas.getBoundingClientRect(); return left() + Math.max(0, Math.min(1, (clientX - box.left) / box.width)) * m.view.span; };
        function schedule() { if (!frame) frame = requestAnimationFrame(() => { frame = 0; draw(); }); }

        function draw() {
            resize();
            c.setTransform(ratio, 0, 0, ratio, 0, 0);
            c.clearRect(0, 0, width, height);
            const band = 17, top = Math.max(44, Math.round((height - band) * .4)), fall = top + band, low = m.floor - 6, high = low + RANGE_DB;
            const y = db => top - 2 - Math.max(0, Math.min(1, (db - low) / (high - low))) * (top - 6);
            c.font = '10px "SDR Display", ui-monospace, monospace';
            c.textBaseline = 'middle';
            c.lineWidth = 1;
            for (let db = Math.ceil(low / 10) * 10; db < high; db += 10) {
                const gy = Math.round(y(db)) + .5;
                c.strokeStyle = COLOR.grid; c.beginPath(); c.moveTo(0, gy); c.lineTo(width, gy); c.stroke();
                if (db % 20 === 0 && gy > 8 && gy < top - 6) { c.fillStyle = COLOR.axis; c.textAlign = 'left'; c.fillText(db + ' dB', 5, gy - 6); }
            }
            c.fillStyle = 'rgba(134,243,223,.05)';
            c.fillRect(0, top, width, band);
            if (!m.view) return;
            const step = TICKS.find(tick => tick / m.view.span * width >= 62) || 1e6, digits = step >= 1e5 ? 1 : step >= 1e4 ? 2 : 3;
            c.textAlign = 'center';
            for (let f = Math.ceil(left() / step) * step; f < left() + m.view.span; f += step) {
                const gx = Math.round(x(f)) + .5;
                c.strokeStyle = COLOR.grid; c.beginPath(); c.moveTo(gx, 0); c.lineTo(gx, top + 4); c.stroke();
                if (gx > 24 && gx < width - 24) { c.fillStyle = COLOR.axis; c.fillText((f / 1e6).toFixed(digits), gx, top + band / 2 + 1); }
            }
            for (let i = 0, gy = fall; i < m.rows.length && gy < height; i++, gy += ROW_HEIGHT) {
                const row = m.rows[i], dx = (row.center - row.span / 2 - left()) / m.view.span * width, dw = row.span / m.view.span * width;
                if (dx < width && dx + dw > 0) c.drawImage(m.history, 0, row.y, BINS, 1, dx, gy, dw, ROW_HEIGHT);
            }
            if (tuned) passband(top, fall);
            trace(m.peak, y, COLOR.peak, 1, false, top);
            trace(m.average, y, COLOR.trace, 1.4, true, top);
            if (tuned) marker(top);
            if (hover !== null) crosshair(y, top);
        }

        function trace(values, y, color, weight, fill, top) {
            const points = [];
            // A pixel covers several bins; keep the strongest so narrow carriers stay visible.
            for (let px = 0; px <= width; px++) {
                const from = Math.min(BINS - 1, Math.floor(px / width * BINS)), to = Math.max(from + 1, Math.floor((px + 1) / width * BINS));
                let db = -200;
                for (let i = from; i < to && i < BINS; i++) db = Math.max(db, values[i]);
                points.push(y(db));
            }
            const path = () => { c.beginPath(); points.forEach((py, px) => px ? c.lineTo(px, py) : c.moveTo(px, py)); };
            if (fill) {
                path();
                c.lineTo(width, top); c.lineTo(0, top); c.closePath();
                const shade = c.createLinearGradient(0, 0, 0, top);
                shade.addColorStop(0, 'rgba(134,243,223,.34)'); shade.addColorStop(1, 'rgba(134,243,223,.02)');
                c.fillStyle = shade; c.fill();
                c.shadowColor = 'rgba(134,243,223,.55)'; c.shadowBlur = 7;
            }
            path();
            c.strokeStyle = color; c.lineWidth = weight; c.lineJoin = 'round';
            c.stroke();
            c.shadowBlur = 0;
        }

        function passband(top, fall) {
            const wide = tuned.bandwidth_hz || 0, f = tuned.frequency_hz;
            const from = tuned.mode === 'usb' ? f : tuned.mode === 'lsb' ? f - wide : f - wide / 2;
            const a = x(from), b = x(from + wide);
            if (b < 0 || a > width || b - a < 1) return;
            c.fillStyle = COLOR.band; c.fillRect(a, 0, b - a, top);
            c.fillStyle = 'rgba(134,243,223,.05)'; c.fillRect(a, fall, b - a, height - fall);
            c.strokeStyle = COLOR.edge; c.lineWidth = 1;
            for (const edge of [a, b]) { const gx = Math.round(edge) + .5; c.beginPath(); c.moveTo(gx, 0); c.lineTo(gx, top); c.stroke(); }
        }

        function marker(top) {
            const gx = Math.round(x(tuned.frequency_hz)) + .5;
            if (gx < 0 || gx > width) return;
            c.strokeStyle = COLOR.marker; c.lineWidth = 1;
            c.beginPath(); c.moveTo(gx, 0); c.lineTo(gx, height); c.stroke();
            c.fillStyle = COLOR.marker;
            c.beginPath(); c.moveTo(gx - 5, 0); c.lineTo(gx + 5, 0); c.lineTo(gx, 7); c.closePath(); c.fill();
            c.beginPath(); c.moveTo(gx - 4, top + .5); c.lineTo(gx + 4, top + .5); c.lineTo(gx, top - 5); c.closePath(); c.fill();
        }

        function crosshair(y, top) {
            const gx = Math.round(hover) + .5, bin = Math.max(0, Math.min(BINS - 1, Math.floor(hover / width * BINS))), hz = left() + hover / width * m.view.span;
            c.strokeStyle = 'rgba(255,255,255,.4)'; c.lineWidth = 1;
            c.setLineDash([3, 3]); c.beginPath(); c.moveTo(gx, 0); c.lineTo(gx, height); c.stroke(); c.setLineDash([]);
            const text = options.label(hz) + '  ' + m.average[bin].toFixed(0) + ' dB', w = c.measureText(text).width + 14, tx = Math.max(3, Math.min(width - w - 3, gx + 8));
            c.fillStyle = COLOR.tip; c.strokeStyle = 'rgba(134,243,223,.4)';
            c.beginPath();
            if (c.roundRect) c.roundRect(tx, 5, w, 19, 4); else c.rect(tx, 5, w, 19);
            c.fill(); c.stroke();
            c.fillStyle = '#d9fff7'; c.textAlign = 'left'; c.fillText(text, tx + 7, 15);
            c.fillStyle = '#fff'; c.beginPath(); c.arc(gx, Math.min(top - 3, y(m.average[bin])), 2.5, 0, Math.PI * 2); c.fill();
        }

        const observer = new ResizeObserver(schedule);
        observer.observe(canvas);
        const point = e => { const box = canvas.getBoundingClientRect(); hover = m.view && e.pointerType !== 'touch' ? Math.max(0, Math.min(box.width, e.clientX - box.left)) : null; schedule(); };
        canvas.addEventListener('pointermove', point, { signal: options.signal });
        canvas.addEventListener('pointerleave', () => { hover = null; schedule(); }, { signal: options.signal });
        schedule();
        return {
            push, frequencyAt,
            retune(tuning) { tuned = tuning; schedule(); },
            edges() { return m.view ? [left(), left() + m.view.span] : null; },
            destroy() { observer.disconnect(); if (frame) cancelAnimationFrame(frame); frame = 0; }
        };
    }

    window.RTLSDRScope = { create, memory };
})();
