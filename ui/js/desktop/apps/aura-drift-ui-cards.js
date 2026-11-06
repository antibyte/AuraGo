/* Aura Drift - menu cards: mode, car, track (map thumbnail), livery swatches, stat bars
   and the arrow-key group helper. Pure DOM builders; the menu shell lives in aura-drift-ui.js,
   which loads first and provides AD.uiKit (el, button, humanize, tr). */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const kit = AD.uiKit;
    if (!kit) return;
    const el = kit.el;
    const cards = AD.uiCards = {};

    /* Stat bars normalise each value against the whole garage, so the weakest car
       still shows a stub and the strongest fills the bar. Top speed is in km/h. */
    const TOP_MS = { vektor: 71, kobalt: 70, lumen: 73, tundra: 68, softeis: 69, maehkoenig: 59, couch: 67, bertha: 72 };
    const STATS = [
        ['power', car => car.power, car => Math.round(car.power * 100)],
        ['grip', car => car.grip, car => Math.round(car.grip * 100)],
        ['mass', car => car.mass, car => Math.round(car.mass * 100)],
        ['top', car => TOP_MS[car.id] || 66, car => Math.round((TOP_MS[car.id] || 66) * 3.6)],
        ['air', car => car.airControl, car => Math.round(car.airControl * 100)]
    ];
    let ranges = null;
    function range(index) {
        if (!ranges) {
            ranges = STATS.map(stat => {
                const values = AD.CARS.map(stat[1]);
                return [Math.min.apply(null, values), Math.max.apply(null, values)];
            });
        }
        return ranges[index];
    }
    cards.statFill = function (index, car) {
        const r = range(index);
        const span = r[1] - r[0] || 1;
        return 0.16 + 0.84 * (STATS[index][1](car) - r[0]) / span;
    };

    /* ---- mode ---------------------------------------------------------- */
    cards.modeCard = function (mode, index, ctx) {
        const b = el('button', 'agdrift-card agdrift-mode-card');
        b.type = 'button';
        b.dataset.mode = mode.id;
        b.appendChild(el('span', 'agdrift-card-no', String(index + 1).padStart(2, '0')));
        b.appendChild(el('span', 'agdrift-card-name', ctx.modeName(mode.id)));
        const desc = ctx.L('aura_drift.mode.' + mode.id + '.desc', '');
        if (desc) b.appendChild(el('span', 'agdrift-card-desc', desc));
        const pips = el('span', 'agdrift-pips');
        pips.setAttribute('aria-hidden', 'true');
        const count = Math.max(1, mode.opponents);
        for (let i = 0; i < count; i++) pips.appendChild(el('i', mode.opponents > 0 ? '' : 'is-solo'));
        b.appendChild(pips);
        return b;
    };

    /* ---- car ----------------------------------------------------------- */
    cards.carCard = function (car, ctx) {
        const b = el('button', 'agdrift-card agdrift-car-card');
        b.type = 'button';
        b.dataset.car = car.id;
        b.style.setProperty('--car', car.colour);
        b.appendChild(el('span', 'agdrift-car-chip'));
        b.appendChild(el('span', 'agdrift-card-name', car.name));
        const tag = ctx.L('aura_drift.car.' + car.id + '.tag', '');
        if (tag) b.appendChild(el('span', 'agdrift-card-tag', tag));
        const stats = el('span', 'agdrift-stats');
        STATS.forEach((stat, i) => {
            const row = el('span', 'agdrift-stat');
            row.appendChild(el('span', 'agdrift-stat-label', ctx.L('aura_drift.stat.' + stat[0])));
            const bar = el('span', 'agdrift-stat-bar');
            const fill = el('i');
            fill.style.setProperty('--fill', cards.statFill(i, car).toFixed(3));
            bar.appendChild(fill);
            row.appendChild(bar);
            row.appendChild(el('span', 'agdrift-stat-value', String(stat[2](car))));
            stats.appendChild(row);
        });
        b.appendChild(stats);
        const quirk = ctx.L('aura_drift.car.' + car.id + '.quirk', '');
        if (quirk) b.appendChild(el('span', 'agdrift-card-quirk', quirk));
        return b;
    };

    cards.liverySwatch = function (car, index, label) {
        const b = el('button', 'agdrift-swatch');
        b.type = 'button';
        b.dataset.livery = String(index);
        b.style.setProperty('--sw', car.liveries[index] || car.colour);
        b.setAttribute('aria-label', label);
        return b;
    };

    /* ---- track --------------------------------------------------------- */
    function mix(hex, other, k) {
        const a = parseInt(hex.slice(1), 16), b = parseInt(other.slice(1), 16);
        const ch = shift => Math.round(((a >> shift) & 255) * (1 - k) + ((b >> shift) & 255) * k);
        return 'rgb(' + ch(16) + ',' + ch(8) + ',' + ch(0) + ')';
    }

    const MAP_W = 176, MAP_H = 84, MAP_DPR = 2;
    cards.drawMap = function (canvas, manifest, biome) {
        canvas.width = MAP_W * MAP_DPR;
        canvas.height = MAP_H * MAP_DPR;
        const c = canvas.getContext('2d');
        if (!c) return;
        c.scale(MAP_DPR, MAP_DPR);
        const pts = AD.trackPoints(manifest);
        let x0 = Infinity, x1 = -Infinity, y0 = Infinity, y1 = -Infinity;
        for (const p of pts) { x0 = Math.min(x0, p[0]); x1 = Math.max(x1, p[0]); y0 = Math.min(y0, p[1]); y1 = Math.max(y1, p[1]); }
        const pad = 12;
        const k = Math.min((MAP_W - pad * 2) / (x1 - x0 || 1), (MAP_H - pad * 2) / (y1 - y0 || 1));
        const ox = (MAP_W - (x1 - x0) * k) / 2 - x0 * k, oy = (MAP_H - (y1 - y0) * k) / 2 - y0 * k;
        const q = pts.map(p => [p[0] * k + ox, p[1] * k + oy]);
        const path = new Path2D();
        const mid = (a, b) => [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
        const start = mid(q[q.length - 1], q[0]);
        path.moveTo(start[0], start[1]);
        for (let i = 0; i < q.length; i++) {
            const m = mid(q[i], q[(i + 1) % q.length]);
            path.quadraticCurveTo(q[i][0], q[i][1], m[0], m[1]);
        }
        path.closePath();
        c.lineJoin = 'round';
        c.lineCap = 'round';
        c.strokeStyle = 'rgba(0,0,0,0.6)';
        c.lineWidth = 9;
        c.stroke(path);
        c.strokeStyle = mix(biome.road, '#ffffff', 0.38);
        c.lineWidth = 6;
        c.stroke(path);
        c.strokeStyle = biome.accent;
        c.lineWidth = 1.4;
        c.setLineDash([3, 4]);
        c.stroke(path);
        c.setLineDash([]);
        c.fillStyle = '#ffffff';
        c.strokeStyle = '#05080f';
        c.lineWidth = 2;
        c.beginPath();
        c.arc(start[0], start[1], 3.6, 0, Math.PI * 2);
        c.stroke();
        c.fill();
    };

    cards.trackCard = function (track, ctx) {
        const biome = (AD.BIOMES && AD.BIOMES[track.biome]) || { road: '#30343a', accent: '#9fd3ff', ground: '#243b2b' };
        const b = el('button', 'agdrift-card agdrift-track-card');
        b.type = 'button';
        b.dataset.track = track.id;
        b.style.setProperty('--acc', biome.accent);
        b.style.setProperty('--ground', biome.ground);
        const map = el('canvas', 'agdrift-map');
        map.setAttribute('aria-hidden', 'true');
        try { cards.drawMap(map, track, biome); } catch (_) { /* a missing 2D context leaves an empty thumbnail */ }
        b.appendChild(map);
        b.appendChild(el('span', 'agdrift-card-name', track.name));
        b.appendChild(el('span', 'agdrift-card-tag', ctx.L('aura_drift.biome.' + track.biome)));
        const blurb = ctx.L('aura_drift.track.' + track.id + '.blurb', '');
        if (blurb) b.appendChild(el('span', 'agdrift-card-desc', blurb));
        const best = el('span', 'agdrift-track-best');
        b.appendChild(best);
        b._best = best;
        return b;
    };

    /* ---- arrow-key groups ---------------------------------------------- */
    /* Roving tabindex: the pressed item takes the Tab stop, arrows move (and with
       `select` also choose) inside the group. Up and Down jump a grid row. */
    cards.roving = function (group, itemSelector, select) {
        const items = () => Array.prototype.filter.call(group.querySelectorAll(itemSelector), n => !n.disabled);
        function sync() {
            const list = items();
            const current = list.find(n => n.getAttribute('aria-pressed') === 'true' || n.getAttribute('aria-selected') === 'true') || list[0];
            for (const n of list) n.tabIndex = n === current ? 0 : -1;
        }
        group.addEventListener('keydown', event => {
            const list = items();
            const at = list.indexOf(document.activeElement);
            if (at < 0 || event.altKey || event.ctrlKey || event.metaKey) return;
            let cols = 1;
            if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
                const template = getComputedStyle(group).gridTemplateColumns;
                cols = template && template !== 'none' ? template.split(' ').length : 1;
            }
            let next = -1;
            switch (event.key) {
            case 'ArrowRight': next = at + 1; break;
            case 'ArrowLeft': next = at - 1; break;
            case 'ArrowDown': next = at + cols; break;
            case 'ArrowUp': next = at - cols; break;
            case 'Home': next = 0; break;
            case 'End': next = list.length - 1; break;
            default: return;
            }
            event.preventDefault();
            event.stopPropagation();
            if (next < 0 || next >= list.length) return;
            list[next].focus();
            if (select) list[next].click();
        });
        group.addEventListener('focusin', sync);
        return sync;
    };
})();
