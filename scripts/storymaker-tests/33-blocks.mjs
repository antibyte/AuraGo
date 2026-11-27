// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
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
