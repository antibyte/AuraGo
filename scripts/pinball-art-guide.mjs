// Art guides for the AuraPinball paintings (ported from the art stream's guide.mjs and plastics-template.mjs).
//   node scripts/pinball-art-guide.mjs playfield <id|all>   playfield layout guide, 887 x 1843 SVG + PNG
//   node scripts/pinball-art-guide.mjs plastics [--check]   plastics sheet template, 1024 x 1536 SVG + PNG + JSON
// Output goes to reports/pinball-guide/ (git-ignored). Add --no-png to skip the headless Chrome rasteriser.
// Keep the colour code and the geometry unchanged: the existing paintings were made over these guides.
// `plastics --check` fails when SHEET in tables/kit/plastics.js differs from the freshly computed template.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath, pathToFileURL } from 'node:url';

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const base = pathToFileURL(path.join(REPO, 'ui/js/desktop/apps/pinball') + path.sep).href;
const OUT = path.join(REPO, 'reports/pinball-guide');
const args = process.argv.slice(2), flags = new Set(args.filter(a => a.startsWith('--'))), pos = args.filter(a => !a.startsWith('--'));
const [command, target] = pos;

// Headless Chrome rasteriser.
function svgToPng(src, out, w, h) {
    const chrome = ['C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe', 'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe', 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', '/usr/bin/google-chrome', '/usr/bin/chromium'].find(p => fs.existsSync(p));
    if (!chrome) { console.log('no Chrome found, PNG skipped: ' + out); return; }
    const html = path.join(os.tmpdir(), 'pbsvg-' + process.pid + '.html');
    fs.writeFileSync(html, `<!doctype html><html><body style="margin:0;background:#000;overflow:hidden"><img src="${pathToFileURL(path.resolve(src)).href}" style="display:block;width:${w}px;height:${h}px"></body></html>`);
    const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'pbsvgp-'));
    spawnSync(chrome, ['--headless=new', `--screenshot=${path.resolve(out)}`, `--window-size=${w},${h}`, '--hide-scrollbars', '--force-device-scale-factor=1', '--no-first-run', '--disable-extensions', `--user-data-dir=${profile}`, pathToFileURL(html).href], { stdio: 'ignore', windowsHide: true, timeout: 60000 });
    try { fs.rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 }); fs.unlinkSync(html); } catch { /* ignore */ }
    console.log(fs.existsSync(out) ? 'png: ' + out : 'FAILED ' + out);
}

// Playfield guide. Colours: outline fill #3a4250, shooter lane #1c2028, apron zone #101216, ramps green, inserts white outlines,
// wall wood/metal/plastic/rubber, posts, bumpers yellow, standups pink, holes black, magnets/discs/orrery/pendulum dashed violet.
async function playfield(id) {
    const { tableOf } = await import(base + 'tables/index.js');
    const { lampPolygon, FIELD_W, FIELD_H } = await import(base + 'lights/shapes.js');
    const L = tableOf(id).layout;
    const W = 887, H = Math.round(W * FIELD_H / FIELD_W), S = W / FIELD_W;
    const px = (x, y) => [((x + FIELD_W / 2) * S).toFixed(1), ((FIELD_H - y) * S).toFixed(1)];
    const pts = list => list.map(p => px(p[0], p[1]).join(',')).join(' ');
    const m = v => (v * S).toFixed(1);
    const out = [];
    out.push(`<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">`);
    out.push(`<rect width="${W}" height="${H}" fill="#000000"/>`);
    const outer = L.walls.find(w => w.id === 'outer');
    if (outer) out.push(`<polygon points="${pts([...outer.points])}" fill="#3a4250"/>`);
    if (L.plunger && L.plunger.laneX) { const a = px(L.plunger.laneX[0], 0.78), b = px(L.plunger.laneX[1], 0); out.push(`<rect x="${a[0]}" y="${a[1]}" width="${(b[0] - a[0]).toFixed(1)}" height="${(b[1] - a[1]).toFixed(1)}" fill="#1c2028"/>`); }
    out.push(`<rect x="0" y="${px(0, 0.03)[1]}" width="${W}" height="${m(0.03)}" fill="#101216"/>`);
    for (const p of L.paths || []) {
        const w = (p.width || 0.04) + 0.006;
        out.push(`<polyline points="${pts(p.points)}" fill="none" stroke="#2fd36b" stroke-opacity="0.55" stroke-width="${m(w)}" stroke-linecap="round" stroke-linejoin="round"/>`);
    }
    for (const lamp of L.lamps || []) {
        if (lamp.mount && lamp.mount !== 'insert') continue;
        out.push(`<polygon points="${pts(lampPolygon(lamp, 20))}" fill="none" stroke="#ffffff" stroke-opacity="0.75" stroke-width="1.6"/>`);
    }
    const WALL = { wood: ['#8a5a2b', 0.012], metal: ['#d0d4da', 0.005], plastic: ['#28c8ff', 0.006], rubber: ['#ff3b30', 0.008] };
    for (const w of L.walls || []) {
        if (w.id === 'outer' || !w.points || w.points.length < 2) continue;
        const k = WALL[w.render] || WALL[w.material]; if (!k || w.render === 'none') continue;
        out.push(`<polyline points="${pts(w.points)}" fill="none" stroke="${k[0]}" stroke-width="${m(k[1])}" stroke-linecap="round" stroke-linejoin="round"/>`);
    }
    if (outer) out.push(`<polyline points="${pts(outer.points)}" fill="none" stroke="#8a5a2b" stroke-width="${m(0.012)}"/>`);
    for (const p of L.posts || []) { const c = px(p.x, p.y); out.push(`<circle cx="${c[0]}" cy="${c[1]}" r="${m(Math.max(0.004, p.r))}" fill="${p.material === 'rubber' ? '#ff3b30' : '#d0d4da'}"/>`); }
    const seg = (a, b, col, w, dash) => { const p = px(a[0], a[1]), q = px(b[0], b[1]); out.push(`<line x1="${p[0]}" y1="${p[1]}" x2="${q[0]}" y2="${q[1]}" stroke="${col}" stroke-width="${m(w)}" stroke-linecap="round"${dash ? ` stroke-dasharray="${dash}"` : ''}/>`); };
    const bar = (x, y, ang, width, col) => { const dx = Math.cos(ang + Math.PI / 2) * width / 2, dy = Math.sin(ang + Math.PI / 2) * width / 2; seg([x - dx, y - dy], [x + dx, y + dy], col, 0.007); };
    for (const d of L.devices || []) {
        if (d.type === 'bumper') { const c = px(d.x, d.y); out.push(`<circle cx="${c[0]}" cy="${c[1]}" r="${m(d.r || 0.03)}" fill="#ffd400"/>`); }
        else if (d.type === 'hole' && !d.hidden) { const c = px(d.x, d.y); out.push(`<circle cx="${c[0]}" cy="${c[1]}" r="${m((d.r || 0.017) * 1.2)}" fill="#000" stroke="#fff" stroke-width="2"/>`); }
        else if (d.type === 'standup') bar(d.x, d.y, d.angle || 0, d.width || 0.02, '#ff38d4');
        else if (d.type === 'dropBank') for (const t of d.targets || []) bar(t.x, t.y, d.angle || 0, d.width || 0.02, '#ff38d4');
        else if (d.type === 'rollover') { const c = px(d.x, d.y); out.push(`<rect x="${(c[0] - S * 0.003).toFixed(1)}" y="${(c[1] - S * 0.012).toFixed(1)}" width="${m(0.006)}" height="${m(0.024)}" rx="3" fill="#ffffff"/>`); }
        else if (d.type === 'spinner' || d.type === 'gate') seg(d.a, d.b, '#b06cff', 0.004);
        else if (d.type === 'magnet' || d.type === 'disc' || d.type === 'orrery') { const c = px(d.x, d.y); out.push(`<circle cx="${c[0]}" cy="${c[1]}" r="${m(d.r || 0.03)}" fill="none" stroke="#b06cff" stroke-width="3" stroke-dasharray="6 5"/>`); }
        else if (d.type === 'pendulum') {
            // Dashed line across the swing: pivot -/+ axis * length (plan units of the pendulum def).
            const pv = d.pivot || [d.x, d.y], ax = d.axis || [1, 0], len = d.length || 0.1, n = Math.hypot(ax[0], ax[1]) || 1;
            seg([pv[0] - ax[0] / n * len, pv[1] - ax[1] / n * len], [pv[0] + ax[0] / n * len, pv[1] + ax[1] / n * len], '#b06cff', 0.004, '6 5');
        }
        else if (d.type === 'captive') { const c = px(d.x, d.y); out.push(`<circle cx="${c[0]}" cy="${c[1]}" r="${m(0.0135)}" fill="#e8e8e8"/>`); }
    }
    for (const f of L.flippers || []) {
        const len = f.length || 0.059, ang = -Math.PI / 6, tip = [f.x + f.side * Math.cos(ang) * len, f.y + Math.sin(ang) * len];
        seg([f.x, f.y], tip, '#ff8a1e', 0.016);
    }
    out.push('</svg>');
    fs.mkdirSync(OUT, { recursive: true });
    const file = path.join(OUT, id + '-guide.svg');
    fs.writeFileSync(file, out.join('\n'));
    console.log(file, W + 'x' + H);
    if (!flags.has('--no-png')) svgToPng(file, path.join(OUT, id + '-guide.png'), W, H);
}

// Plastics template: every cell with its pixel rect and world rect (kit coordinates u = x - cx for the left piece).
async function plasticsCells() {
    const { bandSegments } = await import(base + 'tables/kit/geom.js');
    const W = 1024, H = 1536;
    const cells = [], svg = [];
    svg.push(`<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">`, `<rect width="${W}" height="${H}" fill="#000"/>`);
    // Fit a world rect [x0, y0, x1, y1] (y up) into a pixel cell, uniform scale, centred.
    function fit(world, cell, pad = 14) {
        const [x0, y0, x1, y1] = world, [cx, cy, cw, ch] = cell;
        const s = Math.min((cw - 2 * pad) / (x1 - x0), (ch - 2 * pad) / (y1 - y0));
        const w = (x1 - x0) * s, h = (y1 - y0) * s, ox = cx + (cw - w) / 2, oy = cy + (ch - h) / 2;
        return { s, rect: [ox, oy, w, h].map(v => +v.toFixed(2)), px: (x, y) => [(ox + (x - x0) * s).toFixed(1), (oy + (y1 - y) * s).toFixed(1)] };
    }
    const poly = (f, p) => p.map(q => f.px(q[0], q[1]).join(',')).join(' ');
    { // 1. Slingshot plastic: the three sling posts pushed out by 9 mm, rounded.
        const S = [[-0.1665, 0.360], [-0.1665, 0.2586], [-0.1150, 0.2273]], grow = 0.009;
        const band = bandSegments(S, grow).map(e => e[0]);
        const world = [Math.min(...band.map(p => p[0])) - 0.002, Math.min(...band.map(p => p[1])) - 0.002, Math.max(...band.map(p => p[0])) + 0.002, Math.max(...band.map(p => p[1])) + 0.002];
        const f = fit(world, [30, 30, 330, 760]);
        svg.push(`<polygon points="${poly(f, band)}" fill="#fff" stroke="#fff" stroke-width="${(0.006 * f.s).toFixed(1)}" stroke-linejoin="round"/>`);
        cells.push({ id: 'sling', px: f.rect, world, mirror: true, height: 0.034, note: 'left slingshot plastic; right is mirrored about cx' });
    }
    { // 2. Return-lane plastic: a bent strip over the outlane/inlane divider and the upper inlane guide.
        const line = [[-0.2045, 0.408], [-0.2045, 0.2376], [-0.126, 0.190]], wd = 0.019;
        const world = [-0.2045 - wd / 2 - 0.002, 0.190 - wd / 2 - 0.002, -0.126 + wd / 2 + 0.002, 0.408 + wd / 2 + 0.002];
        const f = fit(world, [380, 30, 300, 760]);
        svg.push(`<polyline points="${poly(f, line)}" fill="none" stroke="#fff" stroke-width="${(wd * f.s).toFixed(1)}" stroke-linecap="round" stroke-linejoin="round"/>`);
        cells.push({ id: 'guide', px: f.rect, world, mirror: true, height: 0.03, note: 'left return-lane plastic; right is mirrored about cx' });
    }
    { // 3. Back corner plastic: the area outside the top arch (arc centre (0, 0.8125), r 0.2575), left side. Absolute x.
        const r = 0.2575, cy = 0.8125, pts = [[-r, cy + 0.02], [-r, 1.07], [-0.10, 1.07]];
        for (let a = 113; a <= 176; a += 3) pts.push([r * 0.992 * Math.cos(a * Math.PI / 180), cy + r * 0.992 * Math.sin(a * Math.PI / 180)]);
        const world = [-r, cy + 0.02, -0.10, 1.07];
        const f = fit(world, [700, 30, 300, 480]);
        svg.push(`<polygon points="${poly(f, pts)}" fill="#fff"/>`);
        cells.push({ id: 'corner', px: f.rect, world, mirror: true, absolute: true, height: 0.04, note: 'left back-corner panel outside the arch; right is mirrored about x = 0' });
    }
    { // 4. Pop bumper cap: a disc (diameter 58 mm).
        const world = [-0.029, -0.029, 0.029, 0.029], f = fit(world, [700, 530, 300, 300]);
        const c = f.px(0, 0);
        svg.push(`<circle cx="${c[0]}" cy="${c[1]}" r="${(0.029 * f.s).toFixed(1)}" fill="#fff"/>`);
        cells.push({ id: 'cap', px: f.rect, world, mirror: false, height: 0.05, note: 'pop bumper cap, centred on the bumper' });
    }
    for (const [k, y] of [[0, 830], [1, 1010]]) { // 5. Two sign plates (60 x 18 mm) for ramp entrances.
        const world = [-0.030, -0.009, 0.030, 0.009], f = fit(world, [40, y, 620, 170]);
        const a = f.px(-0.030, 0.009), b = f.px(0.030, -0.009);
        svg.push(`<rect x="${a[0]}" y="${a[1]}" width="${(b[0] - a[0]).toFixed(1)}" height="${(b[1] - a[1]).toFixed(1)}" rx="${(0.003 * f.s).toFixed(1)}" fill="#fff"/>`);
        cells.push({ id: 'sign' + (k + 1), px: f.rect, world, mirror: false, height: 0.06, note: 'ramp sign plate ' + (k + 1) });
    }
    { // 6. Apron emblem: an oval badge (120 x 46 mm).
        const world = [-0.060, -0.023, 0.060, 0.023], f = fit(world, [40, 1200, 944, 310]);
        const c = f.px(0, 0);
        svg.push(`<ellipse cx="${c[0]}" cy="${c[1]}" rx="${(0.060 * f.s).toFixed(1)}" ry="${(0.023 * f.s).toFixed(1)}" fill="#fff"/>`);
        cells.push({ id: 'badge', px: f.rect, world, mirror: false, height: 0, note: 'apron badge, flat on the apron' });
    }
    svg.push('</svg>');
    return { W, H, cells, svg: svg.join('\n') };
}

async function plastics() {
    const { W, H, cells, svg } = await plasticsCells();
    if (flags.has('--check')) {
        const { SHEET } = await import(base + 'tables/kit/plastics.js');
        const bad = [];
        if (SHEET.w !== W || SHEET.h !== H) bad.push('sheet size');
        for (const c of cells) {
            const s = SHEET.cells[c.id];
            if (!s) { bad.push(c.id + ': missing'); continue; }
            c.px.forEach((v, i) => { if (Math.abs(v - s.px[i]) > 0.02) bad.push(`${c.id}.px[${i}] ${s.px[i]} != ${v}`); });
            c.world.forEach((v, i) => { if (Math.abs(v - s.world[i]) > 1e-6) bad.push(`${c.id}.world[${i}] ${s.world[i]} != ${v}`); });
            if (c.id !== 'badge' && s.height !== undefined && Math.abs(c.height - s.height) > 1e-9) bad.push(`${c.id}.height`);
        }
        for (const id of Object.keys(SHEET.cells)) if (!cells.some(c => c.id === id)) bad.push(id + ': not in template');
        if (bad.length) { console.error('kit/plastics.js differs from the template:\n  ' + bad.join('\n  ')); process.exit(1); }
        console.log('plastics SHEET matches the template (' + cells.length + ' cells)');
        return;
    }
    fs.mkdirSync(OUT, { recursive: true });
    fs.writeFileSync(path.join(OUT, 'plastics-template.svg'), svg);
    fs.writeFileSync(path.join(OUT, 'plastics-template.json'), JSON.stringify({ sheet: [W, H], cells }, null, 2) + '\n');
    console.log(JSON.stringify(cells.map(c => [c.id, c.px])));
    if (!flags.has('--no-png')) svgToPng(path.join(OUT, 'plastics-template.svg'), path.join(OUT, 'plastics-template.png'), W, H);
}

if (command === 'playfield' && target) {
    if (target === 'all') { const { TABLES } = await import(base + 'tables/index.js'); for (const id of Object.keys(TABLES)) await playfield(id); }
    else await playfield(target);
} else if (command === 'plastics') await plastics();
else { console.error('usage: node scripts/pinball-art-guide.mjs playfield <id|all> | plastics [--check] [--no-png]'); process.exit(2); }
