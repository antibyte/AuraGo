// Chibi Sensei loss chart: the training loss as one SVG line and the validation loss as points, built with createElementNS, plus
// the pure layout() that turns metric points into SVG geometry (the Node model test covers it). Above 600 training points the
// line is averaged in buckets. No animation; a sentence under the chart says the same in words. Classic script, publishes P.chart.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const NS = 'http://www.w3.org/2000/svg';
    const MAX_POINTS = 600;
    const WIDTH = 560;
    const HEIGHT = 220;
    const PAD = { left: 48, right: 12, top: 12, bottom: 30 };

    function finite(v) { return typeof v === 'number' ? Number.isFinite(v) : (v !== null && v !== '' && v !== undefined && Number.isFinite(Number(v))); }
    const round1 = v => Math.round(v * 10) / 10;
    const minOf = list => list.reduce((m, v) => (v < m ? v : m), Infinity);
    const maxOf = list => list.reduce((m, v) => (v > m ? v : m), -Infinity);

    // downsample averages consecutive points in buckets so at most max remain; each bucket keeps its last step.
    function downsample(points, max) {
        const limit = max || MAX_POINTS;
        if (points.length <= limit) return points.slice();
        const size = Math.ceil(points.length / limit), out = [];
        for (let i = 0; i < points.length; i += size) {
            const bucket = points.slice(i, i + size);
            out.push({ step: bucket[bucket.length - 1].step, loss: bucket.reduce((a, p) => a + p.loss, 0) / bucket.length, kind: 'train' });
        }
        return out;
    }

    // layout(metrics, {maxSteps, width, height, maxPoints}) -> SVG geometry; no DOM. The summary uses the raw points.
    function layout(metrics, opts) {
        const o = opts || {};
        const width = o.width || WIDTH, height = o.height || HEIGHT;
        const plot = { left: PAD.left, top: PAD.top, right: width - PAD.right, bottom: height - PAD.bottom };
        const clean = (Array.isArray(metrics) ? metrics : [])
            .filter(m => m && finite(m.step) && finite(m.loss))
            .map(m => ({ step: Number(m.step), loss: Number(m.loss), kind: m.kind === 'eval' ? 'eval' : 'train' }))
            .sort((a, b) => a.step - b.step);
        const rawTrain = clean.filter(m => m.kind === 'train');
        const evals = clean.filter(m => m.kind === 'eval');
        const out = { width, height, plot, empty: true, path: '', evalPoints: [], xTicks: [], yTicks: [], summary: null, evalLast: null, trainCount: 0 };
        if (!rawTrain.length && !evals.length) return out;
        const train = downsample(rawTrain, o.maxPoints);
        const shown = train.concat(evals);
        const losses = shown.map(p => p.loss);
        const xMax = Math.max(1, finite(o.maxSteps) ? Number(o.maxSteps) : 0, maxOf(shown.map(p => p.step)));
        let yMin = minOf(losses), yMax = maxOf(losses);
        if (yMax - yMin < 1e-9) { yMin -= 0.5; yMax += 0.5; } else { const m = (yMax - yMin) * 0.08; yMin -= m; yMax += m; }
        if (yMin < 0 && minOf(losses) >= 0) yMin = 0;
        const w = plot.right - plot.left, h = plot.bottom - plot.top;
        const x = s => round1(plot.left + (s / xMax) * w);
        const y = v => round1(plot.top + (1 - (v - yMin) / (yMax - yMin)) * h);
        [0, 0.25, 0.5, 0.75, 1].forEach(f => { const s = Math.round(xMax * f); if (!out.xTicks.some(tk => tk.value === s)) out.xTicks.push({ x: x(s), value: s }); });
        out.yTicks = [0, 1, 2, 3].map(i => { const v = yMin + (yMax - yMin) * i / 3; return { y: y(v), value: v }; });
        out.empty = false;
        out.path = train.map((p, i) => (i ? 'L' : 'M') + x(p.step) + ' ' + y(p.loss)).join(' ');
        out.evalPoints = evals.map(p => ({ x: x(p.step), y: y(p.loss), step: p.step, loss: p.loss }));
        out.trainCount = train.length;
        if (rawTrain.length) out.summary = { first: rawTrain[0].loss, last: rawTrain[rawTrain.length - 1].loss, min: minOf(rawTrain.map(p => p.loss)), steps: rawTrain[rawTrain.length - 1].step };
        if (evals.length) out.evalLast = evals[evals.length - 1].loss;
        return out;
    }

    function summaryText(t, g) {
        const parts = [];
        const dec = v => P.format.decimal(v, 3);
        if (g.summary) parts.push(t('chibi_sensei.chart.summary', { first: dec(g.summary.first), last: dec(g.summary.last), steps: P.format.number(g.summary.steps), min: dec(g.summary.min) }));
        if (g.evalLast != null) parts.push(t('chibi_sensei.chart.summary_eval', { value: dec(g.evalLast) }));
        return parts.join(' ');
    }

    function svg(name, attrs, text) {
        const node = document.createElementNS(NS, name);
        Object.keys(attrs || {}).forEach(k => node.setAttribute(k, String(attrs[k])));
        if (text != null) node.textContent = String(text);
        return node;
    }

    // create(inst) -> {el, update(metrics, maxSteps, noteText), dispose}
    function create(inst) {
        const { el } = P;
        const t = inst.t;
        const box = svg('svg', { class: 'csn-chart-svg', viewBox: '0 0 ' + WIDTH + ' ' + HEIGHT, 'aria-hidden': 'true', focusable: 'false', 'data-csn': 'chart-svg' });
        const summary = el('p', { class: 'csn-chart-summary', dataset: { csn: 'chart-summary' } });
        const note = el('p', { class: 'csn-muted', dataset: { csn: 'chart-note' } });
        const node = el('figure', { class: 'csn-chart', dataset: { csn: 'chart', empty: '1' } },
            el('figcaption', { class: 'csn-chart-head' }, el('strong', { text: t('chibi_sensei.chart.title') }),
                el('span', { class: 'csn-chart-legend' },
                    el('span', { class: 'csn-chart-key', dataset: { kind: 'train' }, text: t('chibi_sensei.chart.train') }),
                    el('span', { class: 'csn-chart-key', dataset: { kind: 'eval' }, text: t('chibi_sensei.chart.eval') }))),
            box, summary, note);

        function update(metrics, maxSteps, noteText) {
            const g = layout(metrics, { maxSteps });
            while (box.firstChild) box.removeChild(box.firstChild);
            if (!g.empty) {
                g.yTicks.forEach(tk => {
                    box.appendChild(svg('line', { class: 'csn-chart-gridline', x1: g.plot.left, x2: g.plot.right, y1: tk.y, y2: tk.y }));
                    box.appendChild(svg('text', { class: 'csn-chart-axis', x: g.plot.left - 6, y: tk.y + 4, 'text-anchor': 'end' }, P.format.decimal(tk.value, 2)));
                });
                g.xTicks.forEach(tk => box.appendChild(svg('text', { class: 'csn-chart-axis', x: tk.x, y: g.plot.bottom + 16, 'text-anchor': 'middle' }, P.format.number(tk.value))));
                box.appendChild(svg('text', { class: 'csn-chart-axis', x: g.plot.right, y: g.height - 1, 'text-anchor': 'end' }, t('chibi_sensei.chart.axis_step')));
                if (g.path) box.appendChild(svg('path', { class: 'csn-chart-train', d: g.path }));
                if (g.trainCount === 1) { const m = /^M(\S+) (\S+)$/.exec(g.path); if (m) box.appendChild(svg('circle', { class: 'csn-chart-dot', cx: m[1], cy: m[2], r: 3 })); }
                g.evalPoints.forEach(p => box.appendChild(svg('circle', { class: 'csn-chart-eval', cx: p.x, cy: p.y, r: 3.5 })));
            }
            summary.textContent = summaryText(t, g);
            summary.hidden = !summary.textContent;
            note.textContent = noteText || (g.empty ? t('chibi_sensei.chart.empty') : '');
            note.hidden = !note.textContent;
            node.dataset.empty = g.empty ? '1' : '0';
        }

        return { el: node, update, dispose() {} };
    }

    P.chart = { create, layout, downsample, MAX_POINTS };
})();
