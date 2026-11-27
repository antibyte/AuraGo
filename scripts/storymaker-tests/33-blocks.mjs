// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { plain, FakeCanvas, digest, readJSON } from '../storymaker-test-support.mjs';
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
    assert.ok(paint(def, timeFor(0.5)).includes('setLineDash([48,96])'));
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
