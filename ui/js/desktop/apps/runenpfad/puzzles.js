/* Runenpfad puzzles: rune_sequence (memory), lever_bridge (switch logic) and ritual_chain (ordered actions on a timer). DOM-free. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['rune_sequence', 'lever_bridge', 'ritual_chain'];
    const RUNE_KINDS = ['fire', 'frost', 'storm', 'magnet'];

    const SHOW_LEAD = 0.5;        // pause before the oracle starts showing
    const SHOW_LIT = 0.7;         // seconds a glyph stays lit
    const SHOW_GAP = 0.25;        // dark pause between two glyphs
    const FAIL_REPLAY = 1.2;      // wait before the oracle replays after a mistake
    const ORACLE_RANGE = 260;
    const STONE_COOL = 0.3;
    const RITUAL_LIMIT = 10;
    const STAIR_STAGGER = 0.15;
    const HINT_RANGE = 420;

    // Hand-checked lever presets. Segments are indexed 0..3, masks list the segments a lever toggles and `on` the
    // initial state of the four segments. Toggles are self-inverse, so each preset is solvable when the set of segments
    // that start off equals the XOR of some subset of masks (verified for every preset in the simulation test).
    const LEVER_PRESETS = [
        { masks: [[0, 1], [1, 2], [2, 3]], on: [false, true, false, true] },
        { masks: [[0, 2], [1, 3], [0, 1]], on: [false, false, false, false] },
        { masks: [[0, 1, 2], [1, 3], [2, 3]], on: [false, true, false, false] },
        { masks: [[0, 3], [1, 2], [0, 1, 2, 3]], on: [true, false, false, true] },
        { masks: [[0], [1, 2], [2, 3]], on: [false, false, true, false] }
    ];

    function P() {
        return root.RunenpfadPhysics;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }
    function fxShake(sim, amount) { P().fxShake(sim, amount); }
    function rand(sim) { return P().rand(sim); }
    function overlap(ax, ay, aw, ah, bx, by, bw, bh) { return P().rectOverlap(ax, ay, aw, ah, bx, by, bw, bh); }

    function has(zone) {
        return ZONE_IDS.indexOf(zone) >= 0;
    }

    function spikePit(ctx, fromX, count) {
        for (let x = fromX; x < fromX + count; x++) {
            ctx.setTile(x, ctx.rows - 1, ctx.world.groundFill, 'solid');
            ctx.hazards.push({ kind: 'spikes', assetId: 'spikes', x: x * ctx.TILE, y: (ctx.rows - 2) * ctx.TILE, w: ctx.TILE, h: ctx.TILE });
        }
    }

    // A column of solid tiles that opens like a rune gate; `puzzleId` ties it to its puzzle.
    function makeColumn(ctx, tx, ty0, ty1, puzzleId) {
        const tiles = [];
        for (let y = ty0; y <= ty1; y++) {
            ctx.setTile(tx, y, ctx.world.groundFill, 'solid');
            tiles.push({ tx: tx, ty: y, tile: ctx.tiles[ctx.tiles.length - 1] });
        }
        const gate = { id: 'gt_' + ctx.gates.length, x: tx * ctx.TILE, y: ty0 * ctx.TILE, w: ctx.TILE, h: (ty1 - ty0 + 1) * ctx.TILE, open: false, tiles: tiles, switchIds: [], puzzleId: puzzleId, portcullis: ty1 - ty0 >= 6 };
        ctx.gates.push(gate);
        return gate;
    }

    // Optional vault: a closed hut (roof is the walkable way over) with a rune chest and treasure behind the gate.
    function buildVault(ctx, x0, puzzleId) {
        const g = ctx.groundY;
        const T = ctx.TILE;
        for (let x = x0; x <= x0 + 4; x++) ctx.setTile(x, g - 3, ctx.world.groundTop, 'solid');
        const gate = makeColumn(ctx, x0, g - 2, g - 1, puzzleId);
        ctx.setTile(x0 + 4, g - 2, ctx.world.groundFill, 'solid');
        ctx.setTile(x0 + 4, g - 1, ctx.world.groundFill, 'solid');
        ctx.chests.push({ x: (x0 + 2.5) * T, y: g * T, opened: false, kind: RUNE_KINDS[((ctx.cfg.id || 1) + 1) % RUNE_KINDS.length] });
        ctx.collectibles.push({ id: 'vt_' + ctx.collectibles.length, kind: 'treasure', assetId: 'treasure', animated: false, x: (x0 + 1.5) * T, y: (g - 0.5) * T, collected: false });
        if (ctx.gemCount() < 3) ctx.placeGem(x0 + 3, g - 1.4);
        return gate;
    }

    // ---- rune_sequence ----

    function buildRuneSequence(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        const n = ctx.cfg.worldIndex <= 1 ? 3 : 4;
        const vault = ctx.variant === 'vault';
        const lastStone = 5.5 + (n - 1) * 3.5;
        const gateX = Math.ceil(lastStone + 3);
        const width = vault ? gateX + 7 : gateX + 3;
        const puzzleId = 'or_' + ctx.extras.oracles.length;
        ctx.fillGround(cx, width, g);
        let gate;
        if (vault) {
            ctx.addPlatformRow(cx + gateX - 2, 2, g - 2, 'platform');
            gate = buildVault(ctx, cx + gateX, puzzleId);
        } else {
            gate = makeColumn(ctx, cx + gateX, 0, g - 1, puzzleId);
        }
        const oracle = { id: puzzleId, x: (cx + 2.5) * T, y: g * T, len: n, seq: [], state: 'idle', step: 0, showIdx: -1, showLit: false, t: 0, hitFlash: 0, failT: 0, gateId: gate.id, vault: vault };
        ctx.extras.oracles.push(oracle);
        const shift = ((ctx.cfg.id || 1) + ctx.zIdx) % 4;
        for (let i = 0; i < n; i++) {
            ctx.extras.glyphStones.push({ id: 'gs_' + ctx.extras.glyphStones.length, oracleId: puzzleId, glyph: (shift + i) % 4, x: (cx + 5.5 + i * 3.5) * T, y: g * T, lit: false, litT: 0, cool: 0, fail: 0 });
        }
        ctx.addCoinLine(cx + 5, 4, g - 2, true);
        if (!vault) ctx.addCoinLine(cx + gateX + 1, 2, g - 1, false);
        ctx.addCheckpoint(cx + 1);
        ctx.addProp(cx + 1, g);
        return width;
    }

    function stonesOf(sim, o) {
        return sim.stage.glyphStones.filter(function (s) { return s.oracleId === o.id; });
    }

    function startShow(sim, o) {
        if (!o.seq.length) {
            const stones = stonesOf(sim, o);
            let prev = -1;
            for (let i = 0; i < o.len; i++) {
                let k = Math.floor(rand(sim) * stones.length);
                if (k === prev) k = (k + 1) % stones.length;
                prev = k;
                o.seq.push(stones[k].glyph);
            }
        }
        o.state = 'showing';
        o.step = 0;
        o.showIdx = -1;
        o.showLit = false;
        o.t = SHOW_LEAD;
        stonesOf(sim, o).forEach(function (s) { s.lit = false; s.litT = 0; });
        sim.notice = 'runenpfad.notice.seq_watch';
    }

    function stepShow(sim, o, dt) {
        o.t -= dt;
        if (o.t > 0) return;
        const stones = stonesOf(sim, o);
        if (o.showLit) {
            stones.forEach(function (s) { s.lit = false; });
            o.showLit = false;
            if (o.showIdx + 1 >= o.seq.length) {
                o.state = 'input';
                o.step = 0;
                o.t = 0;
                return;
            }
            o.t = SHOW_GAP;
            return;
        }
        o.showIdx += 1;
        o.showLit = true;
        o.t = SHOW_LIT;
        const glyph = o.seq[o.showIdx];
        const stone = stones.filter(function (s) { return s.glyph === glyph; })[0];
        if (stone) {
            stone.lit = true;
            pushEvent(sim, 'seq_show', { oracleId: o.id, glyph: glyph, index: o.showIdx, x: stone.x, y: stone.y });
            fxBurst(sim, 'rune', stone.x, stone.y - 30, 6, { color: '#7ee8fa', speed: 90, life: 0.5 });
        }
    }

    function solveSequence(sim, o) {
        o.state = 'solved';
        stonesOf(sim, o).forEach(function (s) { s.lit = true; });
        pushEvent(sim, 'seq_solved', { x: o.x, y: o.y });
        sim.notice = 'runenpfad.notice.seq_solved';
        sim.stageScore += 200;
        fxBurst(sim, 'rune', o.x, o.y - 50, 18, { color: '#7ee8fa', speed: 200, life: 0.9 });
        const gate = sim.stage.gates.filter(function (gt) { return gt.id === o.gateId; })[0];
        const extra = root.RunenpfadZonesExtra;
        if (gate && !gate.open && extra) extra.openGate(sim, gate);
    }

    function hitStone(sim, o, stone) {
        if (stone.cool > 0) return;
        stone.cool = STONE_COOL;
        if (o.state !== 'input') return;
        if (stone.glyph === o.seq[o.step]) {
            stone.lit = true;
            stone.litT = 0.5;
            pushEvent(sim, 'seq_hit', { glyph: stone.glyph, step: o.step, x: stone.x, y: stone.y });
            fxBurst(sim, 'rune', stone.x, stone.y - 30, 10, { color: '#7ee8fa', speed: 130, life: 0.5 });
            o.step += 1;
            if (o.step >= o.seq.length) solveSequence(sim, o);
        } else {
            o.state = 'fail';
            o.t = FAIL_REPLAY;
            o.failT = 0.6;
            o.step = 0;
            stonesOf(sim, o).forEach(function (s) { s.lit = false; s.litT = 0; s.fail = 0.6; });
            pushEvent(sim, 'seq_fail', { x: o.x, y: o.y });
            sim.notice = 'runenpfad.notice.seq_fail';
            fxBurst(sim, 'spark', stone.x, stone.y - 30, 8, { color: '#ff6b7d', speed: 130, life: 0.4 });
        }
    }

    function stepOracles(sim, dt) {
        const p = sim.player;
        sim.stage.oracles.forEach(function (o) {
            if (o.hitFlash > 0) o.hitFlash = Math.max(0, o.hitFlash - dt);
            if (o.failT > 0) o.failT = Math.max(0, o.failT - dt);
            stonesOf(sim, o).forEach(function (s) {
                if (s.cool > 0) s.cool = Math.max(0, s.cool - dt);
                if (s.fail > 0) s.fail = Math.max(0, s.fail - dt);
                if (s.litT > 0) {
                    s.litT -= dt;
                    if (s.litT <= 0 && o.state !== 'solved') s.lit = false;
                }
            });
            if (o.state === 'idle') {
                if (Math.abs(p.x - o.x) < ORACLE_RANGE && Math.abs(p.y - o.y) < 220) startShow(sim, o);
            } else if (o.state === 'showing') {
                stepShow(sim, o, dt);
            } else if (o.state === 'fail') {
                o.t -= dt;
                if (o.t <= 0) startShow(sim, o);
            }
        });
    }

    // ---- lever_bridge ----

    function buildLeverBridge(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        const preset = LEVER_PRESETS[Math.floor(ctx.rng() * LEVER_PRESETS.length)];
        const puzzleId = 'lb_' + ctx.extras.levers.length;
        ctx.fillGround(cx, 10, g);
        spikePit(ctx, cx + 10, 8);
        ctx.fillGround(cx + 18, 5, g);
        for (let i = 0; i < 4; i++) {
            const from = ctx.platforms.length;
            ctx.addPlatformRow(cx + 10 + i * 2, 2, g, 'platform');
            const tiles = ctx.platforms.slice(from);
            tiles.forEach(function (t) {
                t.puzzle = puzzleId;
                t.solid = preset.on[i];
            });
            ctx.extras.bridgeSegs.push({ id: 'bs_' + ctx.extras.bridgeSegs.length, puzzleId: puzzleId, x: (cx + 10 + i * 2) * T, y: g * T, w: 2 * T, on: preset.on[i], tiles: tiles });
        }
        preset.masks.forEach(function (mask, i) {
            ctx.extras.levers.push({ id: 'lv_' + ctx.extras.levers.length, puzzleId: puzzleId, x: (cx + 1.5 + i * 3) * T, y: g * T, on: false, mask: mask.slice(), cool: 0, locked: false });
        });
        ctx.addCoinLine(cx + 11, 6, g - 2, true);
        ctx.collectibles.push({ id: 'lbs_' + ctx.collectibles.length, kind: 'star', assetId: 'star', animated: false, x: (cx + 20.5) * T, y: (g - 1.6) * T, collected: false });
        ctx.addEnemy(ctx.nextEnemyId(), cx + 21, g, cx + 19, cx + 22);
        ctx.addCheckpoint(cx + 1);
        ctx.addProp(cx + 9, g);
        return 23;
    }

    function segsOf(sim, puzzleId) {
        return sim.stage.bridgeSegs.filter(function (s) { return s.puzzleId === puzzleId; });
    }

    function setSeg(sim, seg, on) {
        seg.on = on;
        seg.tiles.forEach(function (t) { t.solid = on; });
        pushEvent(sim, 'bridge_toggle', { x: seg.x + seg.w / 2, y: seg.y, on: on });
        fxBurst(sim, 'rune', seg.x + seg.w / 2, seg.y, 6, { color: on ? '#7ee8fa' : '#94a3b8', speed: 90, life: 0.4 });
    }

    function pullLever(sim, lv) {
        if (lv.locked || lv.cool > 0) return;
        lv.cool = 0.3;
        lv.on = !lv.on;
        pushEvent(sim, 'lever_pull', { x: lv.x, y: lv.y, on: lv.on });
        fxBurst(sim, 'spark', lv.x, lv.y - 34, 6, { color: '#fcd34d', speed: 110, life: 0.3 });
        const segs = segsOf(sim, lv.puzzleId);
        lv.mask.forEach(function (idx) { if (segs[idx]) setSeg(sim, segs[idx], !segs[idx].on); });
        if (segs.every(function (s) { return s.on; })) {
            const mid = segs[0].x + (segs[segs.length - 1].x + segs[segs.length - 1].w - segs[0].x) / 2;
            sim.stage.levers.forEach(function (o) { if (o.puzzleId === lv.puzzleId) o.locked = true; });
            pushEvent(sim, 'bridge_done', { x: mid, y: segs[0].y });
            sim.notice = 'runenpfad.notice.bridge_done';
            sim.stageScore += 150;
            fxBurst(sim, 'rune', mid, segs[0].y - 10, 16, { color: '#7ee8fa', speed: 190, life: 0.8 });
        }
    }

    function stepLevers(sim, dt) {
        const p = sim.player;
        const stage = sim.stage;
        stage.levers.forEach(function (lv) {
            if (lv.cool > 0) lv.cool = Math.max(0, lv.cool - dt);
        });
        if (!stage.leverHinted && stage.levers.some(function (lv) { return Math.abs(p.x - lv.x) < HINT_RANGE && !lv.locked; })) {
            stage.leverHinted = true;
            sim.notice = 'runenpfad.notice.lever_hint';
        }
    }

    // ---- ritual_chain ----

    function buildRitualChain(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        const req = ctx.variant === 'req';
        const puzzleId = 'rt_' + ctx.extras.rituals.length;
        const width = req ? 22 : 24;
        ctx.fillGround(cx, width, g);
        let gate;
        if (req) {
            gate = makeColumn(ctx, cx + 19, 0, g - 1, puzzleId);
        } else {
            ctx.addPlatformRow(cx + 15, 2, g - 2, 'platform');
            gate = buildVault(ctx, cx + 17, puzzleId);
        }
        const stairs = [{ at: 7, row: g - 2 }, { at: 10, row: g - 4 }, { at: 13, row: g - 6 }].map(function (s, i) {
            const from = ctx.platforms.length;
            ctx.addPlatformRow(cx + s.at, 2, s.row, 'platform');
            const tiles = ctx.platforms.slice(from);
            tiles.forEach(function (t) {
                t.puzzle = puzzleId;
                t.solid = false;
            });
            return { x: (cx + s.at) * T, y: s.row * T, w: 2 * T, up: false, k: 0, tiles: tiles };
        });
        const braziers = [
            { x: (cx + 5.2) * T, y: g * T, lit: false, order: 0 },
            { x: (cx + 11.2) * T, y: (g - 4) * T, lit: false, order: 1 },
            { x: (cx + 14.2) * T, y: (g - 6) * T, lit: false, order: 2 }
        ];
        ctx.extras.rituals.push({
            id: puzzleId,
            state: 'idle',
            t: 0,
            timeLimit: RITUAL_LIMIT,
            plate: { x: (cx + 2.5) * T, y: g * T, down: false },
            stairs: stairs,
            braziers: braziers,
            next: 0,
            cool: 0,
            failT: 0,
            gateId: gate.id,
            required: req
        });
        ctx.addCoinLine(cx + 7, 2, g - 3, false);
        ctx.addCoinLine(cx + 10, 2, g - 5, false);
        ctx.addCheckpoint(cx + 1);
        ctx.addProp(cx + 1, g);
        return width;
    }

    function setStair(sim, st, up) {
        st.up = up;
        st.k = up ? 1 : 0;
        st.tiles.forEach(function (t) { t.solid = up; });
    }

    function startRitual(sim, r) {
        r.state = 'active';
        r.t = 0;
        r.next = 0;
        r.plate.down = true;
        pushEvent(sim, 'plate_down', { x: r.plate.x, y: r.plate.y });
        fxBurst(sim, 'dust', r.plate.x, r.plate.y, 6, { color: '#d9c9a8', speed: 80, life: 0.4 });
        sim.notice = 'runenpfad.notice.ritual_hint';
    }

    function failRitual(sim, r) {
        const wasUp = r.stairs.some(function (s) { return s.up; });
        r.state = 'idle';
        r.t = 0;
        r.next = 0;
        r.cool = 0.8;
        r.failT = 0.7;
        r.plate.down = false;
        r.stairs.forEach(function (s) { setStair(sim, s, false); });
        r.braziers.forEach(function (b) { b.lit = false; });
        pushEvent(sim, 'ritual_fail', { x: r.plate.x, y: r.plate.y });
        if (wasUp) pushEvent(sim, 'stairs_down', { x: r.stairs[1].x + r.stairs[1].w / 2, y: r.stairs[1].y });
        sim.notice = 'runenpfad.notice.ritual_fail';
        fxBurst(sim, 'dust', r.plate.x, r.plate.y, 8, { color: '#94a3b8', speed: 100, life: 0.5 });
    }

    function finishRitual(sim, r) {
        r.state = 'done';
        const last = r.braziers[r.braziers.length - 1];
        pushEvent(sim, 'ritual_done', { x: last.x, y: last.y });
        sim.notice = 'runenpfad.notice.ritual_done';
        sim.stageScore += 220;
        fxBurst(sim, 'rune', last.x, last.y - 40, 18, { color: '#fcd34d', speed: 200, life: 0.9 });
        const gate = sim.stage.gates.filter(function (gt) { return gt.id === r.gateId; })[0];
        const extra = root.RunenpfadZonesExtra;
        if (gate && !gate.open && extra) extra.openGate(sim, gate);
    }

    function lightBrazier(sim, r, b) {
        if (r.state !== 'active' || b.lit) return false;
        if (b.order !== r.next) {
            failRitual(sim, r);
            return true;
        }
        b.lit = true;
        r.next += 1;
        pushEvent(sim, 'brazier_lit', { x: b.x, y: b.y, order: b.order });
        fxBurst(sim, 'ember', b.x, b.y - 36, 12, { color: '#fb923c', speed: 110, life: 0.7 });
        if (r.next >= r.braziers.length) finishRitual(sim, r);
        return true;
    }

    function stepRituals(sim, dt) {
        const p = sim.player;
        sim.stage.rituals.forEach(function (r) {
            if (r.cool > 0) r.cool = Math.max(0, r.cool - dt);
            if (r.failT > 0) r.failT = Math.max(0, r.failT - dt);
            if (r.state === 'active') {
                r.t += dt;
                r.stairs.forEach(function (s, i) {
                    if (!s.up && r.t >= i * STAIR_STAGGER) {
                        setStair(sim, s, true);
                        if (i === 0) pushEvent(sim, 'stairs_up', { x: r.stairs[1].x + r.stairs[1].w / 2, y: r.stairs[1].y });
                    }
                });
                if (r.t >= r.timeLimit) failRitual(sim, r);
            } else if (r.state === 'idle' && r.cool <= 0
                && overlap(p.x - 12, p.y - 8, 24, 12, r.plate.x - 30, r.plate.y - 18, 60, 22)) {
                startRitual(sim, r);
            }
            if (!sim.stage.ritualHinted && Math.abs(p.x - r.plate.x) < HINT_RANGE && r.state === 'idle') {
                sim.stage.ritualHinted = true;
                sim.notice = 'runenpfad.notice.ritual_hint';
            }
        });
    }

    // ---- shared hooks ----

    function build(zone, ctx) {
        if (zone === 'rune_sequence') return buildRuneSequence(ctx);
        if (zone === 'lever_bridge') return buildLeverBridge(ctx);
        if (zone === 'ritual_chain') return buildRitualChain(ctx);
        return 0;
    }

    function step(sim, dt) {
        stepOracles(sim, dt);
        stepLevers(sim, dt);
        stepRituals(sim, dt);
    }

    function nearest(list, x, y, w, h, boxFn) {
        let best = null;
        let bestD = Infinity;
        list.forEach(function (o) {
            const b = boxFn(o);
            if (!overlap(x, y, w, h, b[0], b[1], b[2], b[3])) return;
            const d = Math.abs((b[0] + b[2] / 2) - (x + w / 2));
            if (d < bestD) {
                bestD = d;
                best = o;
            }
        });
        return best;
    }

    // Attack boxes, bolts and slam shock: returns true when a puzzle object absorbed the hit.
    function hitRect(sim, x, y, w, h) {
        const stage = sim.stage;
        let hit = false;
        const oracle = nearest(stage.oracles, x, y, w, h, function (o) { return [o.x - 30, o.y - 90, 60, 90]; });
        if (oracle && oracle.state !== 'solved') {
            hit = true;
            oracle.hitFlash = 0.3;
            if (oracle.state === 'idle' || oracle.state === 'input') startShow(sim, oracle);
        }
        const stone = nearest(stage.glyphStones, x, y, w, h, function (s) { return [s.x - 24, s.y - 56, 48, 56]; });
        if (stone) {
            const o = stage.oracles.filter(function (oo) { return oo.id === stone.oracleId; })[0];
            if (o && o.state !== 'solved') {
                hit = true;
                hitStone(sim, o, stone);
            }
        }
        const lever = nearest(stage.levers, x, y, w, h, function (l) { return [l.x - 22, l.y - 56, 44, 56]; });
        if (lever && !lever.locked) {
            hit = true;
            pullLever(sim, lever);
        }
        stage.rituals.forEach(function (r) {
            const b = nearest(r.braziers, x, y, w, h, function (bz) { return [bz.x - 26, bz.y - 64, 52, 64]; });
            if (b && r.state !== 'done') {
                hit = true;
                lightBrazier(sim, r, b);
            }
        });
        return hit;
    }

    function slamLand(sim, x, y) {
        hitRect(sim, x - 90, y - 70, 180, 80);
    }

    // Falling onto a glyph stone counts as a stomp and bounces the hero.
    function applyForces(sim, p) {
        if (p.vy <= 120 || p.onGround || p.onLadder) return;
        sim.stage.glyphStones.forEach(function (s) {
            if (!overlap(p.x - 10, p.y - 4, 20, 14, s.x - 26, s.y - 58, 52, 28)) return;
            const o = sim.stage.oracles.filter(function (oo) { return oo.id === s.oracleId; })[0];
            if (!o || o.state === 'solved') return;
            hitStone(sim, o, s);
            p.vy = -430;
            p.slamActive = false;
            fxBurst(sim, 'spark', p.x, p.y, 6, { color: '#ffe066', speed: 140, arc: Math.PI, angle: -Math.PI / 2, life: 0.3 });
        });
    }

    // Touching a brazier lights it as well.
    function interact(sim) {
        const p = sim.player;
        sim.stage.rituals.forEach(function (r) {
            if (r.state !== 'active') return;
            r.braziers.forEach(function (b) {
                if (r.state === 'active' && !b.lit && overlap(p.x - p.w / 2, p.y - p.h, p.w, p.h, b.x - 24, b.y - 60, 48, 60)) lightBrazier(sim, r, b);
            });
        });
    }

    function lights(sim) {
        const out = [];
        const stage = sim.stage;
        stage.glyphStones.forEach(function (s) {
            out.push({ x: s.x, y: s.y - 30, radius: s.lit ? 120 : 40, intensity: s.lit ? 0.75 : 0.12, flicker: false });
        });
        stage.rituals.forEach(function (r) {
            r.braziers.forEach(function (b) {
                out.push({ x: b.x, y: b.y - 34, radius: b.lit ? 170 : 44, intensity: b.lit ? 0.9 : 0.15, flicker: b.lit });
            });
        });
        return out;
    }

    root.RunenpfadPuzzles = {
        ZONE_IDS: ZONE_IDS,
        LEVER_PRESETS: LEVER_PRESETS,
        has: has,
        build: build,
        step: step,
        applyForces: applyForces,
        interact: interact,
        slamLand: slamLand,
        hitRect: hitRect,
        lights: lights
    };
})(typeof window !== 'undefined' ? window : globalThis);
