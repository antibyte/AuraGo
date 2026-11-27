// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { plain, FakeCanvas, digest, readJSON, renderAssets } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();
const cat = SM.scene.indexCatalog(t.catalog);

check('blocks: indexCatalog indexes the icons', () => {
    const idx = SM.scene.indexCatalog(t.catalog);
    assert.ok(idx.icons['icon.cpu'].paths.length > 0);
    assert.ok(Array.isArray(idx.icons['icon.cpu'].tags));
    assert.deepEqual(Object.keys(SM.scene.indexCatalog({ entries: [] }).icons), []);
});

// ---- C2: model, areas, check, create, serialize and revive

const B = SM.blocks;
const stageOf = (aspect, env = null) => SM.stage.create({ aspect, env });
const BOARD_ENV = { id: 'env.board', kind: 'environment', drawn: 'board' };
const inside = (a, S, eps = 1e-6) => a.x >= S.x - eps && a.y >= S.y - eps && a.x + a.w <= S.x + S.w + eps && a.y + a.h <= S.y + S.h + eps;
const nodes = n => Array.from({ length: n }, (_, i) => ({ id: 'n' + i, label: 'Node ' + i }));
const diagramDef = (n, extra = {}) => ({ id: 'diagram-1', kind: 'diagram', diagram: { layout: 'flow', nodes: nodes(n) }, ...extra });

check('blocks: area gives the safe box, the board rect or the fallback, and null for an unknown name', () => {
    const S = SM.stage.safeBox('16:9'), st = stageOf('16:9'), office = t.byId('env.office');
    assert.deepEqual(plain(B.area(st, null, 'day', 'full')), { ...S, surface: 'panel' });
    const rect = B.BOARD_RECTS['env.office'], board = B.area(st, office, 'day', 'board');
    near(board.x, rect.x + 0.03 * rect.w, 1e-6);
    near(board.w, 0.94 * rect.w, 1e-6);
    near(board.y, rect.y + 0.03 * rect.h, 1e-6);
    near(board.h, 0.94 * rect.h, 1e-6);
    assert.ok(inside(board, S), 'the board inset lies inside the safe box');
    assert.equal(board.surface, 'light');
    assert.equal(B.area(st, { id: 'env.classroom' }, 'night', 'board').surface, 'dark');
    const plainBoard = B.area(st, t.byId('env.plain-stage'), 'day', 'board');
    assert.deepEqual(plain(plainBoard), { x: 960, y: S.y, w: S.x + S.w - 960, h: S.h, surface: 'panel' }, 'the fallback is the right half of the safe box');
    assert.deepEqual(plain(B.area(st, BOARD_ENV, 'day', 'board')), { ...S, surface: 'light' });
    assert.equal(B.area(st, BOARD_ENV, 'night', 'full').surface, 'dark');
    assert.equal(B.area(st, null, 'day', 'nowhere'), null);
    const corner = B.area(st, null, 'day', { x: 0, y: 0 });
    assert.ok(corner && inside(corner, S), 'a point area is shifted into the safe box');
    assert.equal(B.isBoard({ env: BOARD_ENV }), true);
    assert.equal(B.isBoard({ env: office }), false);
    assert.equal(B.isBoard(null), false);
});

check('blocks: in 9:16 the areas lie inside the safe box, left before right, and the board falls back to top', () => {
    const S = SM.stage.safeBox('9:16'), st = stageOf('9:16');
    for (const name of B.AREAS) assert.ok(inside(B.area(st, null, 'day', name), S), name);
    const left = B.area(st, null, 'day', 'left'), right = B.area(st, null, 'day', 'right');
    assert.ok(left.x + left.w < right.x);
    assert.deepEqual(plain(B.area(st, null, 'day', 'board')), plain(B.area(st, null, 'day', 'top')));
    const sq = stageOf('1:1');
    assert.deepEqual(plain(B.area(sq, null, 'day', 'board')), plain(B.area(sq, null, 'day', 'top')));
});

check('blocks: check refuses a 6-node flow, a negative pie value and an unknown icon, and names the fix', () => {
    const flow = B.check(diagramDef(6), cat);
    assert.equal(flow.field, 'diagram.nodes');
    assert.match(flow.message, /2 to 5/);
    const pie = B.check({ id: 'c1', kind: 'chart', chart: { type: 'pie', labels: ['a', 'b'], values: [3, -1] } }, cat);
    assert.equal(pie.field, 'chart.values');
    assert.match(pie.message, /0 or more/);
    const icon = B.check({ id: 'i1', kind: 'icon', icon: 'icon.no-such-thing' }, cat);
    assert.equal(icon.field, 'icon');
    assert.match(icon.message, /icon\.no-such-thing/);
    assert.match(icon.message, /search/i);
    assert.equal(B.check(diagramDef(5), cat), null);
    assert.equal(B.check({ id: 'i1', kind: 'icon', icon: 'icon.cpu', label: 'CPU' }, cat), null);
    assert.equal(B.check({ id: 'd', kind: 'diagram', diagram: { layout: 'flow', nodes: [{ id: 'a', label: 'A' }, { id: 'a', label: 'B' }] } }, cat).field, 'diagram.nodes');
    assert.equal(B.check({ id: 'c', kind: 'chart', chart: { type: 'counter', labels: ['x'], values: [1, 2] } }, cat).field, 'chart.values');
    assert.equal(B.check({ id: 'l', kind: 'list', text: 'a\nb\nc\nd\ne' }, cat).field, 'text');
    assert.equal(B.check({ id: 'h', kind: 'headline', text: 'x'.repeat(81) }, cat).field, 'text');
    assert.equal(B.check(diagramDef(2, { reveals: [1] }), cat).field, 'reveals');
    assert.equal(B.revealSeconds(diagramDef(3)), 0.5 + 0.6 * 3);
    assert.equal(B.revealSeconds({ kind: 'icon' }), 0.8);
    assert.equal(B.revealSeconds({ kind: 'chart' }), 1.2);
    assert.equal(B.revealSeconds({ kind: 'list', text: 'a\nb' }), 1);
    assert.equal(B.revealSeconds({ kind: 'headline', text: 'Ein *wichtiges* Wort' }), 0.4 + 0.15 * 3);
});

check('blocks: create, serialize and revive give an equal state at t = 0, 1, 2', () => {
    const def = diagramDef(3, { area: { x: 100, y: 200, w: 800, h: 400, surface: 'panel' }, t0: 1, dur: 2, entry: 'draw', reveals: [1, 1.5, 2], hidden: true });
    const e = B.create(def);
    assert.equal(e.type, 'block');
    assert.equal(e.x.init, 500);
    assert.equal(e.y.init, 400);
    assert.equal(e.alpha.init, 0);
    assert.equal(e.baseScale, 1);
    e.alpha.add(1, 1.5, 1, 'ease-out');
    e.x.add(1, 1.5, 500, 'ease-out', 400);
    e.marks.add(1.2, 1.8, { node: 'n1', style: 'pulse' });
    e.highlights.add(1.9, 2.4, { style: 'outline' });
    const cam = SM.camera.create(1, 960, 540);
    SM.camera.addZoom(cam, 0.5, 1.5, 1.4, 'ease-in-out');
    SM.camera.addShake(cam, 1, 1.5, 10);
    const wire = JSON.parse(JSON.stringify(B.serialize([SM.scene.createText({ id: 't', text: 'x', x: 0, y: 0 }), e], cam)));
    assert.equal(wire.v, 1);
    assert.equal(wire.blocks.length, 1);
    assert.equal(B.serialize([], cam), null);
    const back = B.revive(wire, cat);
    assert.equal(back.entities.length, 1);
    for (const time of [0, 1, 1.25, 2]) {
        assert.deepEqual(plain(SM.scene.evalEntity(back.entities[0], time)), plain(SM.scene.evalEntity(e, time)), 'entity at ' + time);
        assert.deepEqual(plain(SM.camera.at(back.camera, time)), plain(SM.camera.at(cam, time)), 'camera at ' + time);
    }
    const state = SM.scene.evalEntity(e, 1.25);
    assert.equal(state.block.kind, 'diagram');
    assert.equal(state.time, 1.25);
    assert.equal(state.mark.node, 'n1');
    assert.equal(state.mark.style, 'pulse');
    assert.ok(SM.scene.entityEnd(e) >= 3, 'the end covers t0 + dur');
});

check('blocks: revive refuses 33 blocks, another version, an unknown ease, a long string and a missing number', () => {
    const def = diagramDef(2, { area: { x: 0, y: 0, w: 400, h: 300, surface: 'panel' }, t0: 0, dur: 1 });
    const wire = () => JSON.parse(JSON.stringify(B.serialize([B.create(def)], null)));
    const one = wire();
    assert.notEqual(B.revive(one, cat), null);
    assert.equal(B.revive({ ...one, blocks: new Array(33).fill(one.blocks[0]) }, cat), null);
    assert.equal(B.revive({ ...one, v: 2 }, cat), null);
    const bad = wire();
    bad.blocks[0].alpha.segs.push({ t0: 0, t1: 1, from: 0, to: 1, ease: 'wobble', log: false });
    assert.equal(B.revive(bad, cat), null);
    const long = wire();
    long.blocks[0].block.diagram.nodes[0].label = 'x'.repeat(201);
    assert.equal(B.revive(long, cat), null);
    const nan = wire();
    nan.blocks[0].t0 = null;
    assert.equal(B.revive(nan, cat), null);
    assert.equal(B.revive(null, cat), null);
});

// ---- C3: icon painting

const PALETTE = Object.fromEntries(Object.entries(t.catalog.palettes[0].tokens));
const ICONS = readJSON('ui/img/storymaker/icons/lucide.json');
const realCat = SM.scene.indexCatalog({ ...t.catalog, icon_data: ICONS });
const rcOf = (W, H, aspect = '16:9', c = cat) => SM.render.makeContext({ aspect, fps: 30, palette: PALETTE, cat: c, fontFamilies: { default: 'Geist' }, style: { font: 'default' } }, {}, W, H);
// paint(def, time, {W, H, aspect, cat, area}): the call log of one block at one time
function paint(def, time, o = {}) {
    const W = o.W || 1920, H = o.H || 1080, aspect = o.aspect || '16:9';
    const stage = SM.stage.create({ aspect, width: W, height: H }), rc = rcOf(W, H, aspect, o.cat || cat);
    const area = o.area || B.area(stage, null, 'day', 'full');
    const e = B.create({ t0: 0, dur: 1, ...def, area });
    if (o.setup) o.setup(e);
    const st = SM.scene.evalEntity(e, time), cv = new FakeCanvas(W, H);
    B.draw(cv.ctx, st, rc, stage, stage.baseWindow());
    return cv.ctx.log;
}
// the time at which the eased progress of a 1 s block that starts at 0 is p (ease-out, see itemProgress)
const timeFor = p => 1 - Math.cbrt(1 - p);

check('blocks: every icon draws, one stroke per path and one fill per fill, and the same time gives the same digest', () => {
    for (const icon of ICONS.icons) {
        const log = paint({ id: 'i', kind: 'icon', icon: icon.id, style: 'line' }, 5, { cat: realCat });
        assert.equal(log.filter(l => l.startsWith('stroke(path(')).length, icon.paths.length, icon.id);
        assert.equal(log.filter(l => l.startsWith('fill(path(')).length, (icon.fills || []).length, icon.id);
        assert.ok(log.includes('set lineCap=round') && log.includes('set lineJoin=round'));
    }
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line' }, 5);
    assert.ok(log.includes('set lineWidth=2'), 'the line width is 2 on the 24 grid, scaled by size / 24');
    assert.ok(log.some(l => l.startsWith('scale(')), 'the grid is scaled to the icon size');
    assert.equal(digest(paint({ id: 'i', kind: 'icon', icon: 'icon.cpu' }, 0.4)), digest(paint({ id: 'i', kind: 'icon', icon: 'icon.cpu' }, 0.4)));
});

check('blocks: entry draw reveals the strokes with a dash and ends without one', () => {
    const def = { id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line', entry: 'draw' };
    assert.ok(paint(def, timeFor(0.5)).some(l => l.startsWith('setLineDash([') && l !== 'setLineDash([])'), 'a partial dash while the stroke grows');
    assert.ok(paint(def, 5).includes('setLineDash([])'));
    assert.ok(!paint(def, 5).some(l => l.startsWith('setLineDash([') && l !== 'setLineDash([])'));
    assert.equal(paint(def, 0).filter(l => l.startsWith('stroke(')).length, 0, 'nothing is drawn before the reveal starts');
});

check('blocks: the icon styles line, badge and card draw differently, the card prints its label', () => {
    const logs = ['line', 'badge', 'card'].map(style => paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style, label: 'CPU' }, 5));
    assert.equal(new Set(logs.map(digest)).size, 3);
    assert.ok(logs[2].some(l => l.startsWith('fillText(CPU,0,')), 'the card label is centred');
    assert.ok(!logs[0].some(l => l.startsWith('fillText')), 'the line style prints no label');
    assert.ok(logs[1].some(l => l.startsWith('arc(')), 'the badge is a circle');
    assert.ok(logs[2].some(l => l.startsWith('arcTo(')), 'the card has rounded corners');
    const panel = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line' }, 5);
    assert.ok(panel.some(l => l.includes('set fillStyle=' + PALETTE.bg)), 'a panel surface draws the palette background behind the block');
    const light = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line' }, 5, { area: { ...B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'full'), surface: 'light' } });
    assert.ok(!light.some(l => l.includes('set fillStyle=' + PALETTE.bg)), 'a board surface draws no panel');
});

// ---- C4: diagram layouts, arrows, flow dots, node highlight

const overlap = (a, b) => a.x < b.x + b.w - 1e-6 && b.x < a.x + a.w - 1e-6 && a.y < b.y + b.h - 1e-6 && b.y < a.y + a.h - 1e-6;
const flowNodes = n => Array.from({ length: n }, (_, i) => ({ id: 'n' + i, label: 'Node ' + i, icon: i % 2 ? 'icon.server' : 'icon.cpu' }));
const diagram = (layout, n, extra = {}) => ({ id: 'd1', kind: 'diagram', diagram: { layout, nodes: flowNodes(n), flow: false }, ...extra });

check('blocks: layoutNodes keeps the boxes inside the area and apart, hub 0 in the centre, stack 0 lowest', () => {
    const cases = [['flow', 5], ['cycle', 6], ['hub', 7], ['stack', 5], ['compare', 2], ['flow', 2], ['cycle', 3], ['hub', 3]];
    const areas = [['16:9', 'full'], ['16:9', 'right'], ['9:16', 'full'], ['9:16', 'top'], ['1:1', 'center']];
    for (const [aspect, name] of areas) {
        const stage = SM.stage.create({ aspect }), area = B.area(stage, null, 'day', name);
        for (const [layout, n] of cases) {
            const e = B.create({ ...diagram(layout, n), area }), boxes = B.layoutNodes(SM.scene.evalEntity(e, 0)), label = layout + n + ' in ' + aspect + ' ' + name;
            assert.equal(boxes.length, n, label);
            boxes.forEach((b, i) => {
                assert.ok(inside(b, area, 1e-6), label + ' box ' + i + ' inside');
                assert.ok(b.w > 0 && b.h > 0);
                for (let j = 0; j < i; j++) assert.ok(!overlap(b, boxes[j]), label + ' boxes ' + i + ' and ' + j + ' apart');
            });
            if (layout === 'hub') near(boxes[0].x + boxes[0].w / 2, area.x + area.w / 2, 1e-6, label), near(boxes[0].y + boxes[0].h / 2, area.y + area.h / 2, 1e-6, label);
            if (layout === 'stack') assert.ok(boxes.slice(1).every(b => b.y < boxes[0].y), label + ' node 0 is the lowest');
        }
    }
});

check('blocks: itemProgress of a node is 0 before its reveal and 1 after reveal + ITEM_SPAN', () => {
    const area = { x: 0, y: 0, w: 800, h: 400, surface: 'panel' };
    const e = B.create({ ...diagram('flow', 3), area, t0: 1, dur: 3, reveals: [1, 2, 3.5] });
    const p = (k, time) => B.itemProgress(SM.scene.evalEntity(e, time), k);
    [1, 2, 3.5].forEach((r, k) => {
        assert.equal(p(k, r - 0.01), 0);
        assert.equal(p(k, r), 0);
        assert.ok(p(k, r + B.ITEM_SPAN / 2) > 0 && p(k, r + B.ITEM_SPAN / 2) < 1);
        assert.equal(p(k, r + B.ITEM_SPAN), 1);
    });
    const spread = B.create({ ...diagram('flow', 3), area, t0: 1, dur: 3 });
    const q = (k, time) => B.itemProgress(SM.scene.evalEntity(spread, time), k);
    assert.equal(q(1, 1.99), 0, 'without reveals node k starts at t0 + k * dur / n');
    assert.ok(q(1, 2.2) > 0);
    assert.equal(q(2, 3.5), 1);
});

check('blocks: flow dots are periodic, move with time and stop being drawn once the diagram is not flowing', () => {
    const run = (time, flow = true, setup) => digest(paint(diagram('flow', 3, { diagram: { layout: 'flow', flow, nodes: flowNodes(3) } }), time, { setup }));
    assert.equal(run(4), run(4));
    assert.notEqual(run(4), run(4.4));
    assert.equal(run(4), run(4 + B.FLOW_PERIOD));
    assert.notEqual(run(4), run(4, false));
    assert.equal(run(4, false), run(4.4, false), 'a still diagram does not change with time');
});

check('blocks: an active mark changes only the marked node, and labels never shrink below the floor', () => {
    const base = paint(diagram('flow', 3), 9), mark = style => paint(diagram('flow', 3), 9, { setup: e => e.marks.add(0, 20, { node: 'n1', style }) });
    for (const style of ['pulse', 'outline', 'arrow']) {
        const marked = mark(style);
        assert.notEqual(digest(marked), digest(base), style);
        assert.equal(marked.filter(l => l.startsWith('fillText')).join('\n'), base.filter(l => l.startsWith('fillText')).join('\n'), style + ' prints the same labels');
    }
    const outline = mark('outline');
    let at = 0;
    for (const line of base) { at = outline.indexOf(line, at); assert.ok(at >= 0, 'the unmarked drawing is a subsequence of the marked one'); at++; }
    assert.ok(outline.length > base.length);
    // 5 nodes with 40-character labels in the narrow left area of a 9:16 frame: the font shrinks to the floor and stops there
    const stage = SM.stage.create({ aspect: '9:16', width: 1080, height: 1920 }), floor = B.LABEL_MIN * 1080 / stage.scale;
    const long = Array.from({ length: 5 }, (_, i) => ({ id: 'n' + i, label: 'Ein sehr langer Knotenname Nummer ' + i + ' hier' }));
    const log = paint({ id: 'd', kind: 'diagram', diagram: { layout: 'flow', nodes: long } }, 9, { W: 1080, H: 1920, aspect: '9:16', area: { ...B.area(stage, null, 'day', 'left') } });
    const sizes = log.filter(l => l.startsWith('set font=')).map(l => parseFloat(/(\d+(?:\.\d+)?)px/.exec(l)[1]));
    assert.ok(sizes.length > 0 && sizes.every(s => s >= floor - 1e-6), 'no label below ' + floor + ', got ' + Math.min(...sizes));
});

// ---- C5: charts

const chartDef = (type, values, extra = {}) => ({ id: 'c1', kind: 'chart', chart: { type, labels: values.map((_, i) => 'L' + i), values, ...extra } });

check('blocks: bar heights are proportional to the values at the end and half as high at u = 0.5', () => {
    const def = chartDef('bar', [2, 4, 8], { unit: '%' });
    const heights = time => paint(def, time).filter(l => l.startsWith('fillRect(')).map(l => parseFloat(l.slice(9, -1).split(',')[3]));
    const full = heights(5), half = heights(timeFor(0.5));
    assert.equal(full.length, 3);
    near(full[1] / full[0], 2, 1e-3);
    near(full[2] / full[0], 4, 1e-3);
    full.forEach((h, i) => near(half[i], h * 0.5, 0.01, 'bar ' + i));
    assert.ok(paint(def, 5).some(l => l.startsWith('fillText(8%,')), 'the value is printed with its unit');
    assert.equal(heights(0).reduce((a, b) => a + b, 0), 0, 'nothing has grown at u = 0');
});

check('blocks: a counter shows the eased value rounded to its decimals, plus the unit', () => {
    const def = { ...chartDef('counter', [3.5], { unit: 'ms', title: 'Zeit' }) };
    const area = B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'full');
    const st = SM.scene.evalEntity(B.create({ ...def, area, t0: 0, dur: 1 }), timeFor(0.5)), p = B.itemProgress(st, 0);
    near(p, 0.5, 1e-9);
    const texts = time => paint(def, time).filter(l => l.startsWith('fillText(')).map(l => l.slice(9).split(',')[0]);
    assert.ok(texts(timeFor(0.5)).includes((3.5 * p).toFixed(1) + ' ms'), texts(timeFor(0.5)).join('|'));
    assert.ok(texts(5).includes('3.5 ms'));
    assert.ok(paint(chartDef('counter', [45], { unit: '%' }), 5).some(l => l.startsWith('fillText(45%,')), 'a percent sign follows without a space');
});

check('blocks: a pie is a donut of arcs with an inner radius, a line chart reveals with a dash', () => {
    const arcs = paint(chartDef('pie', [3, 2, 1]), 5).filter(l => l.startsWith('arc(') && !l.includes(',0,6.283'));
    const radii = new Set(arcs.map(l => l.slice(4, -1).split(',')[2]));
    assert.equal(radii.size, 2, 'an outer and an inner radius');
    const [a, b] = [...radii].map(Number).sort((x, y) => x - y);
    assert.ok(a > 0 && a < b);
    assert.equal(arcs.length, 6, 'two arcs per slice');
    assert.ok(paint(chartDef('pie', [3, 2, 1]), 5).some(l => l.startsWith('fillText(L0 50%,')), 'the legend carries the share');
    const line = time => paint(chartDef('line', [1, 3, 2, 5]), time).filter(l => l.startsWith('setLineDash('));
    const mid = line(timeFor(0.5));
    assert.ok(mid.some(l => l !== 'setLineDash([])'), 'a partial dash at u = 0.5');
    assert.ok(line(5).every(l => l === 'setLineDash([])'), 'no dash at the end');
    assert.ok(paint(chartDef('line', [1, 3, 2, 5]), 5).filter(l => l.startsWith('lineTo(')).length >= 3, 'the polyline is drawn');
});

// ---- C6: headline and list

check('blocks: parseHeadline finds the words and the marked word', () => {
    assert.deepEqual(plain(B.parseHeadline('Ein *wichtiges* Wort')), { words: ['Ein', 'wichtiges', 'Wort'], mark: 1 });
    assert.deepEqual(plain(B.parseHeadline('Ohne Markierung')), { words: ['Ohne', 'Markierung'], mark: -1 });
    assert.equal(B.parseHeadline('  *Eins*  ').mark, 0);
    assert.equal(B.check({ id: 'h', kind: 'headline', text: ' * ' }, cat).field, 'text', 'a headline without a word is refused');
});

check('blocks: headline words rise in order and the marked word has an accent marker behind it', () => {
    const def = { id: 'h', kind: 'headline', text: 'Ein *wichtiges* Wort', dur: 1.5 };
    const area = B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'top'), e = B.create({ ...def, area, t0: 0 });
    const prog = time => [0, 1, 2].map(k => B.itemProgress(SM.scene.evalEntity(e, time), k));
    const mid = prog(0.6);
    assert.ok(mid[0] >= mid[1] && mid[1] >= mid[2] && mid[0] > mid[2], 'in order: ' + mid.join(', '));
    assert.deepEqual(prog(0), [0, 0, 0]);
    assert.deepEqual(prog(5), [1, 1, 1]);
    const log = paint(def, 5), at = text => log.findIndex(l => l.startsWith('fillText(' + text + ','));
    assert.deepEqual(['Ein', 'wichtiges', 'Wort'].map(at).every(i => i >= 0), true);
    assert.ok(at('Ein') < at('wichtiges') && at('wichtiges') < at('Wort'));
    const rect = log.findIndex(l => l.startsWith('fillRect(')), fill = log.lastIndexOf('set fillStyle=' + PALETTE.accent, at('wichtiges'));
    assert.ok(rect >= 0 && rect < at('wichtiges') && fill >= 0 && fill < rect, 'the accent rect comes before the marked word');
    assert.equal(paint({ ...def, text: 'Ohne Marker' }, 5).filter(l => l.startsWith('fillRect(')).length, 0);
    assert.equal(paint(def, 0).filter(l => l.startsWith('fillText(')).length, 0, 'no word is printed before its turn');
});

check('blocks: a list reveals its lines in order', () => {
    const def = { id: 'l', kind: 'list', text: 'Laden\nRechnen\nAntworten\nSpeichern', dur: 2 };
    const texts = time => paint(def, time).filter(l => l.startsWith('fillText(')).map(l => l.slice(9).split(',')[0]);
    assert.deepEqual(texts(5), ['Laden', 'Rechnen', 'Antworten', 'Speichern']);
    assert.deepEqual(texts(0), []);
    const area = B.area(SM.stage.create({ aspect: '16:9' }), null, 'day', 'right'), e = B.create({ ...def, area, t0: 0 });
    const prog = time => [0, 1, 2, 3].map(k => B.itemProgress(SM.scene.evalEntity(e, time), k));
    const mid = prog(0.8);
    assert.ok(mid[0] >= mid[1] && mid[1] >= mid[2] && mid[2] >= mid[3] && mid[0] > mid[3], mid.join(', '));
    assert.deepEqual(texts(0.3), ['Laden']);
    assert.equal(paint(def, 5).filter(l => l.startsWith('arc(')).length, 4, 'one bullet per line');
});

// ---- C7: measurement

check('blocks: measure gives a box inside the area and a label of at least LABEL_MIN, halved at scale 0.5', () => {
    const defs = [
        { id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'card', label: 'Prozessor' },
        diagram('flow', 3), diagram('cycle', 4), diagram('hub', 5), diagram('stack', 3), diagram('compare', 2),
        chartDef('bar', [2, 4, 8]), chartDef('pie', [3, 2, 1]), chartDef('line', [1, 3, 2]), chartDef('counter', [45], { unit: '%' }),
        { id: 'h', kind: 'headline', text: 'Ein *wichtiges* Wort' }, { id: 'l', kind: 'list', text: 'Laden\nRechnen\nAntworten' }
    ];
    for (const [aspect, W, H] of [['16:9', 1920, 1080], ['9:16', 1080, 1920]]) {
        const stage = SM.stage.create({ aspect, width: W, height: H }), rc = rcOf(W, H, aspect), win = stage.baseWindow();
        for (const name of ['full', 'right']) {
            const area = B.area(stage, null, 'day', name);
            for (const def of defs) {
                const label = def.kind + (def.diagram ? ' ' + def.diagram.layout : '') + ' ' + aspect + ' ' + name;
                const measureAt = scale => {
                    const e = B.create({ ...def, area, t0: 0, dur: 1 });
                    e.scale.init = scale;
                    return B.measure(new FakeCanvas(W, H).ctx, SM.scene.evalEntity(e, 5), rc, stage, win);
                };
                const m = measureAt(1), half = measureAt(0.5);
                assert.ok(inside(m.box, area, 1e-6), label + ' box inside the area: ' + JSON.stringify(m.box));
                assert.ok(m.box.w > 0 && m.box.h > 0, label);
                if (m.labelPx > 0) {
                    assert.ok(m.labelPx >= B.LABEL_MIN * Math.min(W, H) - 1e-6, label + ' label ' + m.labelPx);
                    near(half.labelPx, m.labelPx / 2, 1e-6, label + ' halves');
                    near(half.box.w, m.box.w / 2, 1e-6, label);
                } else assert.ok(def.kind === 'icon' || def.kind === 'chart', label + ' has a label');
            }
        }
    }
    const stage = SM.stage.create({ aspect: '16:9' }), rc = rcOf(1920, 1080), area = B.area(stage, null, 'day', 'right');
    const text = def => B.measure(new FakeCanvas(1920, 1080).ctx, SM.scene.evalEntity(B.create({ ...def, area, t0: 0, dur: 1 }), 5), rc, stage, stage.baseWindow()).box;
    assert.ok(text({ id: 'h', kind: 'headline', text: 'Kurz' }).w < area.w * 0.5, 'a headline box is the text bounds, not the area');
    assert.ok(text({ id: 'l', kind: 'list', text: 'Eins\nZwei' }).h < area.h * 0.5, 'a list box is the text bounds, not the area');
});

// ---- C8: render and raster draw blocks

check('blocks: drawScene paints a block after the shapes and before the texts, and a block can carry an outline highlight', () => {
    const stage = SM.stage.create({ aspect: '16:9', width: 1920, height: 1080 }), rc = rcOf(1920, 1080), area = B.area(stage, null, 'day', 'center');
    const sceneOf = extra => {
        const block = B.create({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line', entry: 'draw', area, t0: 0, dur: 1 });
        if (extra) extra(block);
        return {
            id: 's1', env: null, variant: 'day', camera: SM.camera.create(1, 960, 540), stages: {},
            entities: [SM.scene.createText({ id: 't', text: 'Hallo', x: 960, y: 900 }), block, SM.scene.createShape({ id: 'r', kind: 'rect', x: 300, y: 300, w: 100, h: 100, color: '$accent' })]
        };
    };
    const run = (time, extra) => { const cv = new FakeCanvas(1920, 1080); SM.render.drawScene(cv.ctx, sceneOf(extra), time, rc); return cv.ctx.log; };
    const log = run(1), at = prefix => log.findIndex(l => l.startsWith(prefix));
    const shape = at('fillRect(-50,-50,100,100)'), block = at('stroke(path('), text = at('fillText(Hallo');
    assert.ok(shape >= 0 && block > shape && text > block, 'shape ' + shape + ', block ' + block + ', text ' + text);
    assert.equal(digest(run(1)), digest(run(1)));
    assert.notEqual(digest(run(0.2)), digest(run(1)), 'the reveal changes the drawing');
    const marked = run(1, e => e.highlights.add(0, 5, { style: 'outline' }));
    assert.ok(marked.some(l => l.startsWith('strokeRect(')), 'an outline highlight frames the block');
    assert.equal(run(1, e => e.alpha.init = 0).filter(l => l.startsWith('stroke(path(')).length, 0, 'a block with alpha 0 is not drawn');
});

check('blocks: cacheKey only follows the scene time while the block animates', () => {
    const area = { x: 0, y: 0, w: 800, h: 400, surface: 'panel' };
    const key = (def, time) => B.cacheKey(SM.scene.evalEntity(B.create({ ...def, area, t0: 0, dur: 1 }), time));
    const icon = { id: 'i', kind: 'icon', icon: 'icon.cpu' };
    assert.notEqual(key(icon, 0.3), key(icon, 0.6), 'during the reveal');
    assert.equal(key(icon, 3), key(icon, 4), 'after the reveal');
    const flow = { ...diagram('flow', 3), diagram: { layout: 'flow', flow: true, nodes: flowNodes(3) } };
    assert.notEqual(key(flow, 3), key(flow, 4), 'flow dots keep moving');
    assert.equal(key(diagram('flow', 3), 3), key(diagram('flow', 3), 4));
    const e = B.create({ ...diagram('flow', 3), area, t0: 0, dur: 1 });
    e.marks.add(2, 6, { node: 'n1', style: 'pulse' });
    assert.notEqual(B.cacheKey(SM.scene.evalEntity(e, 3)), B.cacheKey(SM.scene.evalEntity(e, 4)), 'a running mark');
    assert.notEqual(key(icon, 3), key({ ...icon, icon: 'icon.server' }, 3));
});

// ---- C10: the board backdrop

check('blocks: drawBoard fills the day board with palette.bg and the night board with palette.fg, with 900 grain dots at alpha 0.008', () => {
    const run = (variant, W = 1920, H = 1080) => { const cv = new FakeCanvas(W, H); B.drawBoard(cv.ctx, { variant, palette: PALETTE, W, H }); return cv.ctx.log; };
    const day = run('day'), night = run('night');
    assert.equal(day[day.indexOf('fillRect(0,0,1920,1080)') - 1], 'set fillStyle=' + PALETTE.bg);
    assert.equal(night[night.indexOf('fillRect(0,0,1920,1080)') - 1], 'set fillStyle=' + PALETTE.fg);
    assert.ok(day.includes('set fillStyle=' + PALETTE.fg) && night.includes('set fillStyle=' + PALETTE.bg), 'the dots take the other colour');
    for (const log of [day, night]) {
        const grain = log.slice(log.indexOf('set globalAlpha=0.008'));
        assert.equal(log.filter(l => l.startsWith('arc(')).length, 900);
        assert.equal(grain.filter(l => l.startsWith('set globalAlpha=')).join(), 'set globalAlpha=0.008,set globalAlpha=1', 'every grain dot is drawn at alpha 0.008');
        assert.equal(grain.filter(l => l === 'fill()').length, 900);
    }
    assert.equal(digest(run('day')), digest(run('day')));
    assert.notEqual(digest(run('day')), digest(run('night')));
    const radius = log => Math.max(...log.filter(l => l.startsWith('arc(')).map(l => parseFloat(l.slice(4, -1).split(',')[2])));
    near(radius(run('day', 960, 540)), radius(day) / 2, 0.01, 'the dot radius follows the width');
});

check('blocks: a storyboard scene on env.board draws the board over the backdrop, one on env.office does not', () => {
    const tl = (env, variant) => SM.compile({ scenes: [{ id: 'a', silent: true, environment: env, variant, beats: [{ do: 'show', text: 'Hallo', at: 'center', entry: 'none' }] }] }, {}, t.catalog).timeline;
    const run = (env, variant) => { const cv = new FakeCanvas(1920, 1080); SM.renderFrame(tl(env, variant), 10, cv.ctx, renderAssets()); return cv.ctx.log; };
    const board = run('env.board', 'day'), full = log => log.flatMap((l, i) => l === 'fillRect(0,0,1920,1080)' ? [i] : []);
    assert.equal(full(board).length, 2, 'the backdrop and the board');
    assert.equal(board[full(board)[1] - 1], 'set fillStyle=' + PALETTE.bg);
    assert.ok(board.includes('set globalAlpha=0.008'));
    assert.ok(board.some(l => l.startsWith('fillText(Hallo,')), 'the text is drawn over the board');
    const night = run('env.board', 'night');
    assert.equal(night[full(night)[1] - 1], 'set fillStyle=' + PALETTE.fg);
    assert.ok(!run('env.office', 'day').includes('set globalAlpha=0.008'));
    const plan = SM.planTiming({ scenes: [{ id: 'a' }, { id: 'b', environment: 'env.board', variant: 'night' }, { id: 'c', kind: 'code' }, { id: 'd', environment: 'env.office' }] }, {}, t.catalog).scenes;
    assert.deepEqual(['a', 'b', 'c', 'd'].map(id => [plan[id].environment, plan[id].variant]), [['env.plain-stage', 'day'], ['env.board', 'night'], ['env.board', 'day'], ['env.office', 'day']], 'a scene keeps the environment of the one before it');
});

// ---- C11: text fitting splits at whitespace

check('blocks: a long card label shrinks instead of breaking mid-word', () => {
    const area = { x: 0, y: 0, w: 400, h: 400, surface: 'panel' };
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'card', label: 'Warteschlangen' }, 5, { area });
    const texts = log.filter(l => l.startsWith('fillText(')).map(l => l.slice(9).split(',')[0]);
    assert.deepEqual(texts, ['Warteschlangen'], 'the whole word is on one line: ' + texts.join('|'));
});

// ---- C12: pie legend, list and compare fit narrow areas

// fitted(def, aspect, at) = {area, plan, texts}: the plan box and every painted text as {s, x, y, px} in block coordinates
function fitted(def, aspect, at) {
    const W = aspect === '9:16' ? 1080 : 1920, H = aspect === '9:16' ? 1920 : 1080;
    const stage = SM.stage.create({ aspect, width: W, height: H }), rc = rcOf(W, H, aspect), area = B.area(stage, null, 'day', at);
    const st = SM.scene.evalEntity(B.create({ t0: 0, dur: 1, ...def, area }), 5), cv = new FakeCanvas(W, H);
    const pl = B.plan(cv.ctx, st, rc, stage);
    B.draw(cv.ctx, st, rc, stage, stage.baseWindow());
    let px = 0;
    const texts = [];
    for (const l of cv.ctx.log) {
        if (l.startsWith('set font=')) px = parseFloat(/(\d+(?:\.\d+)?)px/.exec(l)[1]);
        const m = /^fillText\((.*),(-?[\d.]+),(-?[\d.]+)\)$/.exec(l);
        if (m) texts.push({ s: m[1], x: +m[2], y: +m[3], px });
    }
    return { area, st, plan: pl, texts };
}
const insideArea = (r, what) => {
    const { box } = r.plan, hw = r.area.w / 2, hh = r.area.h / 2;
    assert.ok(box.x >= -hw - 1e-6 && box.y >= -hh - 1e-6 && box.x + box.w <= hw + 1e-6 && box.y + box.h <= hh + 1e-6, what + ': plan box ' + JSON.stringify(box));
    for (const x of r.texts) assert.ok(x.y - x.px / 2 >= -hh - 0.5 && x.y + x.px / 2 <= hh + 0.5, what + ': text "' + x.s + '" at y ' + x.y + ' (' + x.px + 'px) leaves +-' + hh);
};
const CASES = [['16:9', 'right'], ['9:16', 'top']];

check('blocks: pie legends fit the area for 4 and 6 slices at 16:9 right and 9:16 top', () => {
    for (const [aspect, at] of CASES) for (const n of [4, 6]) {
        const values = Array.from({ length: n }, (_, i) => i + 1);
        insideArea(fitted(chartDef('pie', values, { labels: values.map((_, i) => 'Anteil ' + i) }), aspect, at), 'pie ' + n + ' ' + aspect + ' ' + at);
    }
});

check('blocks: a list of two-line items stays in the area, at its label floor or above', () => {
    const text = ['Erster Punkt mit einem langen Satz', 'Zweiter Punkt mit einem langen Satz', 'Dritter Punkt mit einem langen Satz', 'Vierter Punkt mit einem langen Satz'].join('\n');
    for (const [aspect, at] of CASES) {
        const r = fitted({ id: 'l', kind: 'list', text }, aspect, at);
        insideArea(r, 'list ' + aspect + ' ' + at);
        assert.ok(r.plan.labelPx > 0);
    }
});

check('blocks: a compare diagram with two-line headers and three two-line points keeps its text inside each node', () => {
    const node = (id, k) => ({ id, label: 'Kopfzeile ' + k + ' mit mehr Text', points: ['Erster langer Punkt ' + k, 'Zweiter langer Punkt ' + k, 'Dritter langer Punkt ' + k] });
    for (const [aspect, at] of CASES) {
        const r = fitted({ id: 'd', kind: 'diagram', diagram: { layout: 'compare', nodes: [node('a', 1), node('b', 2)] } }, aspect, at);
        insideArea(r, 'compare ' + aspect + ' ' + at);
        const boxes = B.layout('compare', 2, r.area.w, r.area.h).boxes;
        for (const x of r.texts) {
            const b = boxes.find(o => x.x >= o.x && x.x <= o.x + o.w);
            assert.ok(b && x.y - x.px / 2 >= b.y - 0.5 && x.y + x.px / 2 <= b.y + b.h + 0.5, 'compare ' + aspect + ': "' + x.s + '" y ' + x.y + ' outside its node ' + JSON.stringify(b));
        }
    }
});

// ---- C13: untrusted block payloads and icon lookups are bounded

check('blocks: revive refuses a segment with t1 < t0, unsorted segments, log ease from 0, and a __proto__ key; it keeps only known block fields', () => {
    const def = diagramDef(2, { area: { x: 0, y: 0, w: 400, h: 300, surface: 'panel' }, t0: 0, dur: 1 });
    const wire = () => JSON.parse(JSON.stringify(B.serialize([B.create(def)], null)));
    const seg = (t0, t1, from, to, log = false) => ({ t0, t1, from, to, ease: 'linear', log });
    const withSegs = segs => { const w = wire(); w.blocks[0].alpha.segs = segs; return B.revive(w, cat); };
    assert.notEqual(withSegs([seg(0, 1, 0, 1), seg(1, 2, 1, 0)]), null, 'sorted segments pass');
    assert.equal(withSegs([seg(2, 1, 0, 1)]), null, 't1 < t0');
    assert.equal(withSegs([seg(1, 2, 0, 1), seg(0, 1, 1, 0)]), null, 'segments out of order');
    assert.equal(withSegs([seg(0, 1, 0, 1, true)]), null, 'log ease from 0');
    assert.equal(withSegs([seg(0, 1, 1, -2, true)]), null, 'log ease to a negative value');
    assert.notEqual(withSegs([seg(0, 1, 1, 2, true)]), null, 'log ease between positive values passes');
    const proto = wire();
    proto.blocks[0].block = JSON.parse(JSON.stringify(proto.blocks[0].block).slice(0, -1) + ',"__proto__":{"x":1}}');
    assert.ok(Object.keys(proto.blocks[0].block).includes('__proto__'), 'the payload really has an own __proto__ key');
    assert.equal(B.revive(proto, cat), null, 'a __proto__ key');
    const extra = wire();
    extra.blocks[0].block.secret = { deep: 1 };
    extra.blocks[0].block.diagram.nodes[0].extra = 5;
    const back = B.revive(extra, cat);
    assert.notEqual(back, null);
    assert.deepEqual(Object.keys(back.entities[0].block).sort(), ['diagram', 'id', 'kind'], 'unknown block fields are dropped');
    assert.deepEqual(Object.keys(back.entities[0].block.diagram.nodes[0]).sort(), ['id', 'label'], 'unknown node fields are dropped');
});

check('blocks: an icon path named like an Object property draws as a path', () => {
    const odd = SM.scene.indexCatalog({ ...t.catalog, icon_data: { schema: 'storymaker-icons/1', icons: [{ id: 'icon.odd', paths: ['constructor', 'toString'], tags: [] }] } });
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.odd', style: 'line' }, 5, { cat: odd });
    assert.deepEqual(log.filter(l => l.startsWith('stroke(')), ['stroke(path(constructor))', 'stroke(path(toString))']);
});

// ---- C14: even stroke reveal, list items slide in from the left

check('blocks: pathLength measures lines exactly and curves and arcs approximately', () => {
    const len = d => B.pathLength(d);
    near(len('M0 0L10 0'), 10, 1e-9);
    near(len('M0 0l-3-4'), 5, 1e-9, 'relative, packed signs');
    near(len('M0 0h10v10z'), 20 + Math.hypot(10, 10), 1e-9, 'h, v and the closing line');
    near(len('M0 0L1.5.5'), Math.hypot(1.5, 0.5), 1e-9, 'packed decimals');
    const circle = len('M0 10A10 10 0 1 0 20 10A10 10 0 1 0 0 10Z');
    assert.ok(Math.abs(circle - 62.8) <= 0.05 * 62.8, 'circle ' + circle);
    const curve = len('M0 0C0 10 10 10 10 0');
    assert.ok(curve > 10 && curve < 30, 'cubic ' + curve);
    assert.equal(len('M0 0A5 5 0 01 10 0'), len('M0 0A5 5 0 0 1 10 0'), 'packed arc flags');
    assert.equal(len(''), 0);
    assert.equal(len('constructor'), 0);
    assert.equal(len(null), 0);
});

check('blocks: every stroke of an icon is revealed by the same share of its own length', () => {
    const log = paint({ id: 'i', kind: 'icon', icon: 'icon.cpu', style: 'line', entry: 'draw' }, timeFor(0.5), { cat: realCat });
    const strokes = [];
    log.forEach((l, i) => {
        const m = /^stroke\(path\((.*)\)\)$/.exec(l);
        if (m) strokes.push({ d: m[1], dash: log[i - 1].slice(13, -2).split(',').map(Number) });
    });
    assert.ok(strokes.length >= 3, 'the icon has several strokes');
    for (const s of strokes) {
        const len = B.pathLength(s.d);
        near(s.dash[1], len, 0.001, s.d);
        near(s.dash[0], len * 0.5, 0.001, s.d);
    }
    assert.ok(new Set(strokes.map(s => s.dash[1])).size > 1, 'the strokes differ in length, so do their dashes');
});

check('blocks: list items slide in from the left', () => {
    const def = { id: 'l', kind: 'list', text: 'Laden\nRechnen', dur: 2 };
    const xOf = (time, s) => parseFloat(paint(def, time).find(l => l.startsWith('fillText(' + s + ',')).slice(10 + s.length).split(',')[0]);
    assert.ok(xOf(timeFor(0.3), 'Laden') < xOf(5, 'Laden'), 'the first line is left of its final place while it slides');
});
