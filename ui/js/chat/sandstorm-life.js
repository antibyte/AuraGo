(function () {
    'use strict';
    if (window.AuraGoSandstormLife) return;   // loaded twice: the running instance keeps its state

    // ═══════════════════════════════════════════════════════════════
    //  SANDSTORM DESERT LIFE
    //
    //  Companion of sandstorm-particles.js. The engine loads it on demand
    //  (loadDesertLife) and drives it from its single render loop:
    //    update(dt, time, geom)   once per frame, before anything is drawn
    //    draw(ctx, geom, stage)   once per scene stage, in painter's order:
    //                               -1 sky, before the far dune
    //                                0 after the far dune
    //                                1 after the mid dune
    //                                2 after the near dune
    //    resize(geom) / reset(time)   layout change / start and stop
    //  A creature is drawn in the stage of the dune it lives on, so the dunes
    //  painted afterwards hide whatever is behind them for free.
    //
    //  What lives here: a sandworm, heat shimmer on the horizon, sand wisps off
    //  the crests, vultures circling in the upper sky (stage -1), a tumbleweed and
    //  a beetle on the mid ridge (stage 1) and, rarely, a caravan on the far
    //  ridge (stage 0). Every family has one update and one draw function.
    //
    //  Nothing here schedules anything: no animation frames, no timers, no
    //  clock of its own. Time is the engine's `time` (ms) and `dt` (frames at
    //  60 Hz), so a test can drive the layer with synthetic frames.
    //  Units: CSS px, y grows downward, speeds in px/s, durations in s.
    //  Harness aids: stats() reports what is alive, forceWorm / forceTumbleweed /
    //  forceBeetle / forceCaravan start a creature on the next update, debug.where()
    //  says where the first of each family is (and where every vulture and the
    //  thermal they circle are), and debug.ridges = true strokes every ridge
    //  polyline in green.
    // ═══════════════════════════════════════════════════════════════

    // ─── Budgets ───
    const MAX_SPRAY = 320;        // sand grains in the air, the oldest is dropped when full
    const MAX_PUFFS = 24;         // soft dust discs
    const MAX_WISPS = 140;        // sand streaks blowing off the crests
    const MAX_TUMBLEWEEDS = 2;    // balls of twigs rolling at once
    const MAX_CAMELS = 6;         // a caravan has 4-6
    const VULTURE_SLOTS = 3;      // 2 birds over a narrow chat, 3 over a wide one

    // ─── Tuning ───
    const TWO_PI = Math.PI * 2;
    const SUN_DIR_X = 0.62;       // the light of the dune flanks in the engine: upper right
    const SUN_DIR_Y = -0.78;
    const STORM_FADE = 0.85;      // creatures lose this much alpha in a full storm
    const GRAVITY = 280;          // px/s^2 for flying sand

    const VULTURE_GLIDE_IN = 3.0;       // s from the screen edge to the thermal
    const VULTURE_FLEE_AT = 0.08;       // storm intensity that sends the birds away
    const VULTURE_FLEE_SPEED = 220;     // px/s, reached by a steady push of VULTURE_FLEE_PUSH ...
    const VULTURE_FLEE_PUSH = 160;      // px/s^2  ... so crossing a 1440 px screen takes about 6.5 s (it took 9 s)
    const VULTURE_RETURN_MIN = 6;       // s of calm they wait after a storm ...
    const VULTURE_RETURN_MAX = 12;      // ... before they glide back in: the calm windows are only 12-24 s long
    const VULTURE_RELOCATE = 15;        // s in view between two looks at where the chat bubbles leave the sky free
    const THERMAL_DRIFT = 24;           // px/s, the most the thermal (and the flock with it) moves to a better spot
    const THERMAL_SWAY = 0.02;          // the whole thermal sways this share of the width to and fro (a period of 90 s)
    const THERMAL_SAMPLES = 12;         // points on each ellipse that tell how much of it is behind the chat
    const THERMAL_TRIES = 8;            // random centres tried besides the current and the default one ...
    const THERMAL_BIAS = 0.08;          // ... which must beat those by this much (about one sample in twelve)
    const FLAP_TIME = 1.2;              // one flap burst: 3 beats
    const WEED_GRAVITY = 420;           // px/s^2 for a hopping tumbleweed
    const BEETLE_DIG = 0.6;             // s to sink into / rise out of the sand
    const BEETLE_DEPTH = 8;             // px below the ridge when buried
    const BEETLE_GAIT = 0.09;           // s between the two tripod leg sets
    const CARAVAN_FADE = 2.0;           // s to fade in out of the haze, and to dissolve in a storm
    const CRITTER_CALM_MS = 6000;       // a beetle or caravan only starts with this much calm ahead

    const WORM_SEGMENTS = 26;
    const WORM_STEP = 1 / (WORM_SEGMENTS + 10);   // path distance between two neighbouring segments
    const WORM_BODY = WORM_SEGMENTS * WORM_STEP;  // path length of the whole body
    const TREMOR_TRAVEL = 3.0;    // the hump rides along the ridge ...
    const TREMOR_SINK = 0.35;     // ... and sinks during the last part of that ride
    const TREMOR_STILL = 0.4;     // silence before the breach
    const SETTLE_TIME = 4.0;
    const RIPPLE_TIME = 0.8;

    // ═══════════════════════════════════════════════════════════════
    //  SMALL HELPERS
    // ═══════════════════════════════════════════════════════════════

    function rand(min, max) {
        return min + Math.random() * (max - min);
    }

    function clamp(v, lo, hi) {
        return v < lo ? lo : (v > hi ? hi : v);
    }

    function lerp(a, b, t) {
        return a + (b - a) * t;
    }

    function smoothstep(a, b, x) {
        const t = clamp((x - a) / (b - a), 0, 1);
        return t * t * (3 - 2 * t);
    }

    function finiteOr(v, fallback) {
        return typeof v === 'number' && isFinite(v) ? v : fallback;
    }

    function makeCanvas(w, h) {
        const c = document.createElement('canvas');
        c.width = Math.max(1, Math.ceil(w));
        c.height = Math.max(1, Math.ceil(h));
        return c;
    }

    // Fixed pools: the slot under the cursor is always the oldest one, so a
    // full pool overwrites its oldest particle and nothing is ever allocated.
    function makePool(max, make) {
        const items = [];
        for (let i = 0; i < max; i++) items.push(make());
        return { items: items, head: 0 };
    }

    function takeOldest(pool) {
        const p = pool.items[pool.head];
        pool.head = (pool.head + 1) % pool.items.length;
        return p;
    }

    // Emitters that must not steal from living particles ask for a free slot.
    function takeFree(pool) {
        const n = pool.items.length;
        for (let k = 0; k < n; k++) {
            const i = (pool.head + k) % n;
            if (!pool.items[i].on) {
                pool.head = (i + 1) % n;
                return pool.items[i];
            }
        }
        return null;
    }

    function countLive(pool) {
        let n = 0;
        for (let i = 0; i < pool.items.length; i++) if (pool.items[i].on) n++;
        return n;
    }

    function clearPool(pool) {
        for (let i = 0; i < pool.items.length; i++) pool.items[i].on = false;
        pool.head = 0;
    }

    // ─── State ───
    const spray = makePool(MAX_SPRAY, function () {
        return { on: false, layer: 0, x: 0, y: 0, vx: 0, vy: 0, t: 0, life: 1, size: 1 };
    });
    const puffs = makePool(MAX_PUFFS, function () {
        return { on: false, layer: 0, x: 0, y: 0, t: 0, life: 1, r0: 1, r1: 2 };
    });
    const wisps = makePool(MAX_WISPS, function () {
        return { on: false, layer: 0, x: 0, y: 0, vx: 0, vy: 0, t: 0, life: 1 };
    });

    const debug = { ridges: false };   // true: stroke every ridge polyline, to check where creatures stand

    let geom = null;              // the engine's geometry object, kept so force*() can act between frames
    let started = false;          // update() has run since the last reset
    let startTime = null;         // engine time handed to reset(), else the first update's time
    let lastTime = 0;             // engine time of the latest update
    let dtFrames = 0;             // this frame in 60 Hz frames (the engine's dt)
    let sdt = 0;                  // ... and in seconds
    let broken = false;           // one failure switches the layer off instead of killing the engine's loop

    let worm = null;              // the running event, null between events
    let wormCount = 0;            // finished events since the last reset
    let nextWormTime = 0;         // engine ms
    let wormRequest = null;       // forceWorm() that arrived before the first update

    let shimmerAmax = 0;          // current heat-shimmer amplitude in CSS px, 0 while off
    let band = null;              // scratch copy of the shimmering band

    let crestList = [];           // { layer, x, carry } for the far and mid dune
    let crestStamp = -Infinity;   // weatherTime of the latest crest scan
    let windExcess = 0;           // how far the wind is above its quiet level
    let spawnRotor = 0;           // rotates which crest gets a free wisp slot first

    // Creatures: schedule clocks (engine ms) and what force*() asked for before the next update.
    let nextTumbleTime = 0;
    let nextBeetleTime = 0;
    let nextCaravanTime = 0;
    let prevStormActive = false;  // the storm flag of the previous frame, to catch a storm's start
    const requests = { tumbleweed: [], beetle: null, caravan: null };

    // How far each dune moved up or down this frame (scroll parallax); sand in the
    // air and dust follow it so they never hang behind a ridge that slid away.
    const layerTopPrev = [NaN, NaN, NaN];
    const layerShift = [0, 0, 0];

    let puffSprite = null;

    // ═══════════════════════════════════════════════════════════════
    //  SCHEDULE HELPERS
    // ═══════════════════════════════════════════════════════════════

    // The first worm shows up 28-50 s after the start, afterwards one every 75-150 s.
    function firstWormTime(from) {
        return from + rand(28000, 50000);
    }

    function nextWormAfter(time) {
        return time + rand(75000, 150000);
    }

    // First tumbleweed 15-35 s after the start, afterwards one every 45-110 s.
    function firstTumbleTime(from) {
        return from + rand(15000, 35000);
    }

    function nextTumbleAfter(time) {
        return time + rand(45000, 110000);
    }

    // First beetle 20-40 s after the start, afterwards one every 60-140 s.
    function firstBeetleTime(from) {
        return from + rand(20000, 40000);
    }

    function nextBeetleAfter(time) {
        return time + rand(60000, 140000);
    }

    // First caravan 80-140 s after the start, afterwards one every 240-420 s.
    function firstCaravanTime(from) {
        return from + rand(80000, 140000);
    }

    function nextCaravanAfter(time) {
        return time + rand(240000, 420000);
    }

    // No storm now and none due within `ms`.
    function calmFor(g, ms) {
        const calmLeft = finiteOr(g.calmLeft, Infinity);
        return !g.stormActive && g.stormIntensity < 0.05 && calmLeft >= ms;
    }

    // The worm needs a long calm stretch: no storm now and none due within 9 s.
    function calmEnough(g) {
        return calmFor(g, 9000);
    }

    // ─── Sprites ───

    function getPuffSprite() {
        if (puffSprite) return puffSprite;
        const c = makeCanvas(64, 64);
        const g = c.getContext('2d');
        if (!g) return null;
        const grad = g.createRadialGradient(32, 32, 0, 32, 32, 32);
        // a bell-shaped falloff: a straight ramp would show a hard core when scaled up
        grad.addColorStop(0, 'rgba(196, 150, 100, 0.28)');
        grad.addColorStop(0.35, 'rgba(196, 150, 100, 0.2)');
        grad.addColorStop(0.7, 'rgba(196, 150, 100, 0.07)');
        grad.addColorStop(1, 'rgba(196, 150, 100, 0)');
        g.fillStyle = grad;
        g.fillRect(0, 0, 64, 64);
        puffSprite = c;
        return c;
    }

    // One lit sphere, drawn once per event and stamped for every body segment.
    function buildSegmentSprite(R, dpr) {
        const box = 2 * R * 1.15;                       // the head is drawn at 1.15 R
        const size = Math.max(8, Math.ceil(box * dpr));
        const c = makeCanvas(size, size);
        const g = c.getContext('2d');
        if (!g) return null;
        const half = size / 2;
        const lx = size * (0.5 + 0.22 * SUN_DIR_X);
        const ly = size * (0.5 + 0.22 * SUN_DIR_Y);
        const grad = g.createRadialGradient(lx, ly, 0, lx, ly, half);
        grad.addColorStop(0, 'rgb(176, 124, 78)');
        grad.addColorStop(0.55, 'rgb(104, 66, 38)');
        grad.addColorStop(1, 'rgb(38, 22, 12)');
        g.fillStyle = grad;
        g.beginPath();
        g.arc(half, half, half, 0, TWO_PI);
        g.fill();
        return c;
    }

    // ═══════════════════════════════════════════════════════════════
    //  SAND SPRAY AND DUST PUFFS
    //  Shared by the worm and by the beetle's burrow.
    // ═══════════════════════════════════════════════════════════════

    function emitSpray(layer, x, y, vx, vy, life, size) {
        const p = takeOldest(spray);
        p.on = true;
        p.layer = layer;
        p.x = x;
        p.y = y;
        p.vx = vx;
        p.vy = vy;
        p.t = 0;
        p.life = life;
        p.size = size;
    }

    function emitPuff(g, layer, x, R) {
        const p = takeFree(puffs) || takeOldest(puffs);
        p.on = true;
        p.layer = layer;
        p.x = x;
        p.y = g.ridgeAt(layer, x) - rand(0, 0.5) * R;
        p.t = 0;
        p.life = rand(2.4, 3.6);
        p.r0 = 0.6 * R;
        p.r1 = 2.4 * R;
    }

    // A burst of sand thrown up from the ridge at x (worm entry and dive).
    function emitBurst(g, w, x) {
        const px = x + rand(-0.5, 0.5) * w.R;
        emitSpray(w.e, px, g.ridgeAt(w.e, px) - 1,
            rand(-90, 90) + g.windDirection * g.windSpeed * 12, rand(-170, -60),
            rand(0.6, 1.2), rand(1, 2.5));
    }

    function updateGrains(g) {
        for (let i = 0; i < spray.items.length; i++) {
            const p = spray.items[i];
            if (!p.on) continue;
            p.t += sdt;
            if (p.t >= p.life) { p.on = false; continue; }
            p.vy += GRAVITY * sdt;
            p.x += p.vx * sdt;
            p.y += p.vy * sdt + layerShift[p.layer];
            // a grain that falls back into its dune is gone
            if (p.y > g.ridgeAt(p.layer, p.x) + 4) p.on = false;
        }
        const drift = g.windDirection * g.windSpeed * 6;
        for (let i = 0; i < puffs.items.length; i++) {
            const p = puffs.items[i];
            if (!p.on) continue;
            p.t += sdt;
            if (p.t >= p.life) { p.on = false; continue; }
            p.x += drift * sdt;
            p.y += -10 * sdt + layerShift[p.layer];
        }
    }

    function drawPuffs(ctx, layer, fade) {
        const sprite = getPuffSprite();
        if (!sprite) return;
        for (let i = 0; i < puffs.items.length; i++) {
            const p = puffs.items[i];
            if (!p.on || p.layer !== layer) continue;
            const age = p.t / p.life;
            const a = 0.9 * Math.pow(1 - age, 1.4) * fade;
            if (a < 0.004) continue;
            const r = lerp(p.r0, p.r1, age);
            ctx.globalAlpha = a;
            ctx.drawImage(sprite, p.x - r, p.y - r, 2 * r, 2 * r);
        }
        ctx.globalAlpha = 1;
    }

    function drawSpray(ctx, layer, fade) {
        ctx.fillStyle = 'rgb(232, 204, 156)';
        for (let i = 0; i < spray.items.length; i++) {
            const p = spray.items[i];
            if (!p.on || p.layer !== layer) continue;
            const a = 0.85 * (1 - p.t / p.life) * fade;
            if (a < 0.02) continue;
            ctx.globalAlpha = a;
            ctx.fillRect(p.x - p.size / 2, p.y - p.size / 2, p.size, p.size);
        }
        ctx.globalAlpha = 1;
    }

    // ═══════════════════════════════════════════════════════════════
    //  OCCLUDERS
    //  The scene canvas lies behind the chat, so whatever stands where a message
    //  bubble sits is not seen. Creatures that take a stand for a while (the worm,
    //  the beetle, the caravan, the vultures' thermal) pick their place with these
    //  rectangles in mind. The rectangles are read once per spawn (the vultures:
    //  once per glide-in and every VULTURE_RELOCATE seconds), never per frame, and
    //  a DOM that cannot be read just means "nothing is hidden".
    // ═══════════════════════════════════════════════════════════════

    const occ = { n: 0, x0: [], y0: [], x1: [], y1: [] };   // scene-canvas rectangles, reused

    function addOccluder(r, originX, originY, clip, yTop, yBottom) {
        let x0 = r.left - originX, y0 = r.top - originY, x1 = r.right - originX, y1 = r.bottom - originY;
        if (clip) {                                  // scrolled out of the chat box: not on screen
            x0 = Math.max(x0, clip.left - originX);
            y0 = Math.max(y0, clip.top - originY);
            x1 = Math.min(x1, clip.right - originX);
            y1 = Math.min(y1, clip.bottom - originY);
        }
        if (!(x1 > x0 && y1 > y0) || y1 <= yTop || y0 >= yBottom) return;   // empty, or off the band
        const i = occ.n++;
        occ.x0[i] = x0;
        occ.y0[i] = y0;
        occ.x1[i] = x1;
        occ.y1[i] = y1;
    }

    // Fills `occ` with the chat bubbles and the robot mascot that touch the horizontal band
    // yTop..yBottom (scene px).
    function collectOccluders(g, yTop, yBottom) {
        occ.n = 0;
        try {
            const cv = g.canvas;
            if (!cv || typeof cv.getBoundingClientRect !== 'function') return occ;
            const origin = cv.getBoundingClientRect();
            const box = document.getElementById('chat-box');
            const clip = box ? box.getBoundingClientRect() : null;
            const bubbles = document.querySelectorAll('#chat-content .msg-row .bubble');
            for (let i = 0; i < bubbles.length; i++) {
                addOccluder(bubbles[i].getBoundingClientRect(), origin.left, origin.top, clip, yTop, yBottom);
            }
            // the round avatar badge beside a bubble hides a creature just as well
            const avatars = document.querySelectorAll('#chat-content .msg-row .avatar');
            for (let i = 0; i < avatars.length; i++) {
                addOccluder(avatars[i].getBoundingClientRect(), origin.left, origin.top, clip, yTop, yBottom);
            }
            const mascot = document.getElementById('chat-robot-mascot');
            if (mascot) addOccluder(mascot.getBoundingClientRect(), origin.left, origin.top, null, yTop, yBottom);
        } catch (err) {
            occ.n = 0;
        }
        return occ;
    }

    function isHidden(x, y) {
        for (let i = 0; i < occ.n; i++) {
            if (x >= occ.x0[i] && x <= occ.x1[i] && y >= occ.y0[i] && y <= occ.y1[i]) return true;
        }
        return false;
    }

    // ═══════════════════════════════════════════════════════════════
    //  SANDWORM
    //    tremor  a hump rides along the ridge, sand trickles, faint rings spread
    //    breach  the body arcs out of the dune at x0 and dives back at x1
    //    settle  dust and spray finish, two dark patches mark the spots
    // ═══════════════════════════════════════════════════════════════

    const hump = { x: 0, h: 0 };                 // scratch: where the tremor hump is and how high
    const P0 = { x: 0, y: 0 };                   // scratch: the segment being drawn
    const T0 = { x: 0, y: 0 };                   // scratch: two points on the path around it ...
    const T1 = { x: 0, y: 0 };
    const TAN = { x: 0, y: 0 };                  // ... and the direction they give

    // The emergence ridge must be clearly above the next nearer ridge, or the
    // worm would rise from behind a dune that is painted over it.
    function wormSpotVisible(g, e, x0, x1) {
        if (e + 1 >= g.layers) return true;
        for (let k = 0; k <= 4; k++) {
            const x = x0 + (x1 - x0) * k / 4;
            if (g.ridgeAt(e, x) > g.ridgeAt(e + 1, x) - 10) return false;
        }
        return true;
    }

    // One candidate path: it enters at x0, runs `dir` for one span and dives at x1. The
    // direction flips when there is no room on that side. The answer lands in `aim`.
    const aim = { x0: 0, x1: 0, dir: 1 };

    function aimWorm(g, span, x0, dirWanted) {
        const lo = 24, hi = g.width - 24;
        let dir = dirWanted || (Math.random() < 0.5 ? -1 : 1);
        let x1 = clamp(x0 + dir * span, lo, hi);
        if (Math.abs(x1 - x0) < span * 0.4) {
            dir = -dir;
            x1 = clamp(x0 + dir * span, lo, hi);
        }
        aim.x0 = x0;
        aim.x1 = x1;
        aim.dir = dir;
    }

    // How much of a candidate worm sits behind chat bubbles, 0 (all in the clear) to 11 (all
    // hidden). The entry and the dive point weigh 2 each, the seven points along the arch
    // (the top edge of the body, one head radius above the path) 1 each.
    function wormHiddenScore(g, e, x0, x1, H, R) {
        let score = 0;
        if (isHidden(x0, g.ridgeAt(e, x0))) score += 2;
        if (isHidden(x1, g.ridgeAt(e, x1))) score += 2;
        for (let k = 1; k <= 7; k++) {
            const s = k / 8;
            const x = x0 + (x1 - x0) * s;
            const y = g.ridgeAt(e, x) - H * Math.pow(Math.sin(Math.PI * s), 0.85) - R;
            if (isHidden(x, y)) score += 1;
        }
        return score;
    }

    function beginWorm(g, opts) {
        const o = opts || {};
        const e = clamp(Math.round(finiteOr(o.layer, Math.random() < 0.8 ? 1 : 0)), 0, Math.min(1, g.layers - 1));
        const S = (e === 1 ? 1.0 : 0.45) * rand(0.85, 1.15);
        const span = clamp(0.20 * g.width, 160, 420) * S;
        const H = clamp(0.26 * g.height, 120, 260) * S;
        const R = 0.17 * H;
        const fixedDir = o.dir < 0 ? -1 : (o.dir > 0 ? 1 : 0);
        let x0 = 0, x1 = 0, dir = 1;
        if (typeof o.x === 'number' && isFinite(o.x)) {
            aimWorm(g, span, clamp(o.x, 24, g.width - 24), fixedDir);       // a fixed spot is never second-guessed
            x0 = aim.x0; x1 = aim.x1; dir = aim.dir;
        } else {
            // The worm is drawn behind the chat, so the best spot is one the bubbles leave free.
            // 8 candidates; a ridge that is hidden behind the next dune is a hard no, then the
            // lowest hidden score wins and the earliest candidate on a tie.
            collectOccluders(g, g.groundLine - 1.3 * H - R, g.groundLine);
            let okScore = Infinity, okX0 = 0, okX1 = 0, okDir = 1;
            let anyScore = Infinity, anyX0 = 0, anyX1 = 0, anyDir = 1;
            for (let attempt = 0; attempt < 8; attempt++) {
                aimWorm(g, span, rand(0.14, 0.86) * g.width, fixedDir);
                const score = occ.n ? wormHiddenScore(g, e, aim.x0, aim.x1, H, R) : 0;
                if (score < anyScore) { anyScore = score; anyX0 = aim.x0; anyX1 = aim.x1; anyDir = aim.dir; }
                if (score < okScore && wormSpotVisible(g, e, aim.x0, aim.x1)) {
                    okScore = score; okX0 = aim.x0; okX1 = aim.x1; okDir = aim.dir;
                    if (score === 0) break;                                  // nothing beats a spot in the clear
                }
            }
            if (okScore < Infinity) { x0 = okX0; x1 = okX1; dir = okDir; }
            else { x0 = anyX0; x1 = anyX1; dir = anyDir; }                   // no spot clears the ridge: the least hidden one
        }
        worm = {
            phase: 'tremor', t: 0, age: 0,
            e: e, S: S, dir: dir, x0: x0, x1: x1, H: H, R: R,
            D: 4.2 * (0.8 + 0.2 * S),                      // length of the breach
            humpFrom: clamp(x0 - dir * 0.12 * g.width, 8, g.width - 8),
            sHead: 0,
            sprite: buildSegmentSprite(R, g.dpr),
            ripples: [], rippleClock: RIPPLE_TIME, trickle: 0,
            entry: 0, dive: 0, divePuffs: false
        };
    }

    // The head's position along the arch, s in [0, 1]. Recomputed from the live
    // ridge every frame, so scroll parallax and dune drift carry the worm along.
    function wormPoint(g, w, s, out) {
        const x = w.x0 + (w.x1 - w.x0) * s;
        out.x = x;
        out.y = g.ridgeAt(w.e, x) - w.H * Math.pow(Math.max(0, Math.sin(Math.PI * s)), 0.85);
    }

    function wormTangent(g, w, s, out) {
        wormPoint(g, w, clamp(s - 0.012, 0.001, 0.999), T0);
        wormPoint(g, w, clamp(s + 0.012, 0.001, 0.999), T1);
        const dx = T1.x - T0.x, dy = T1.y - T0.y;
        const len = Math.hypot(dx, dy) || 1;
        out.x = dx / len;
        out.y = dy / len;
    }

    function humpAt(w) {
        hump.x = lerp(w.humpFrom, w.x0, smoothstep(0, 1, w.t / TREMOR_TRAVEL));   // ease in and out
        const grow = smoothstep(0, 0.4, w.t);
        const sink = 1 - smoothstep(TREMOR_TRAVEL - TREMOR_SINK, TREMOR_TRAVEL, w.t);
        hump.h = grow * sink;
    }

    function updateWorm(g, time) {
        if (!worm) {
            if (wormRequest) {
                const request = wormRequest;
                wormRequest = null;
                beginWorm(g, request);
            } else if (time >= nextWormTime) {
                if (g.layers >= 2 && calmEnough(g)) beginWorm(g, null);
                else nextWormTime = time + 2000;          // blocked: look again in 2 s
            }
        }
        if (!worm) return;
        const w = worm;
        w.t += sdt;
        w.age += sdt;
        if (w.phase === 'tremor') {
            updateTremor(g, w);
        } else if (w.phase === 'breach') {
            updateBreach(g, w);
        } else if (w.t >= SETTLE_TIME) {
            worm = null;
            wormCount += 1;
            nextWormTime = nextWormAfter(time);
        }
    }

    function updateTremor(g, w) {
        humpAt(w);
        for (let i = w.ripples.length - 1; i >= 0; i--) {
            w.ripples[i].t += sdt;
            if (w.ripples[i].t >= RIPPLE_TIME) w.ripples.splice(i, 1);
        }
        w.rippleClock += sdt;
        if (hump.h > 0.3 && w.rippleClock >= RIPPLE_TIME) {   // a faint ring every 0.8 s
            w.rippleClock = 0;
            w.ripples.push({ x: hump.x, t: 0 });
        }
        // Sand trickles down the hump's flanks.
        w.trickle += 2.5 * w.S * dtFrames * hump.h;
        while (w.trickle >= 1) {
            w.trickle -= 1;
            const side = Math.random() < 0.5 ? -1 : 1;
            const u = rand(0.45, 0.95);                       // how far out on the flank, 0 centre .. 1 foot
            const x = hump.x + side * u * 1.2 * w.R;
            const top = g.ridgeAt(w.e, x) - 0.12 * w.R * hump.h - 0.275 * w.R * hump.h * Math.sqrt(1 - u * u);
            emitSpray(w.e, x, top - 0.5, side * rand(6, 26), rand(-22, 6), rand(0.35, 0.7), rand(1, 1.8));
        }
        if (w.t >= TREMOR_TRAVEL + TREMOR_STILL) {
            w.phase = 'breach';
            w.t = 0;
            w.sHead = 0;
            w.ripples.length = 0;
            for (let k = 0; k < 5; k++) emitPuff(g, w.e, w.x0 + rand(-0.8, 0.8) * w.R, w.R);
        }
    }

    function updateBreach(g, w) {
        w.sHead = clamp(w.t / w.D, 0, 1) * (1 + WORM_BODY);
        // Entry: sand falls and sprays at x0 while any segment is still crossing the first 8 % of the path.
        if (w.sHead > 0 && w.sHead < 0.08 + WORM_BODY) {
            w.entry += 6 * w.S * dtFrames;
            while (w.entry >= 1) { w.entry -= 1; emitBurst(g, w, w.x0); }
        }
        // Dive: the head has passed 90 % of the path, then the body follows it under at x1.
        if (w.sHead > 0.9 && w.sHead < 1 + WORM_BODY) {
            w.dive += 10 * w.S * dtFrames;
            while (w.dive >= 1) { w.dive -= 1; emitBurst(g, w, w.x1); }
        }
        if (!w.divePuffs && w.sHead >= 0.92) {
            w.divePuffs = true;
            for (let k = 0; k < 7; k++) emitPuff(g, w.e, w.x1 + rand(-0.8, 0.8) * w.R, w.R);
        }
        if (w.t >= w.D) {
            w.phase = 'settle';
            w.t = 0;
        }
    }

    // ─── Worm drawing ───

    // Everything the worm does happens above the ridge it comes from: the part
    // that is still "inside" the dune is simply never painted.
    function clipAboveRidge(ctx, g, layer) {
        ctx.beginPath();
        ctx.moveTo(0, 0);
        for (let x = 0; x < g.width; x += 6) ctx.lineTo(x, g.ridgeAt(layer, x));
        ctx.lineTo(g.width, g.ridgeAt(layer, g.width));
        ctx.lineTo(g.width, 0);
        ctx.closePath();
        ctx.clip();
    }

    function drawWorm(ctx, g, w, fade) {
        ctx.save();
        try {
            clipAboveRidge(ctx, g, w.e);
            ctx.globalAlpha = fade;
            if (w.phase === 'tremor') drawHump(ctx, g, w);
            else if (w.phase === 'breach') drawBody(ctx, g, w);
            else drawDisturbed(ctx, g, w);
        } finally {
            ctx.restore();
        }
    }

    function drawHump(ctx, g, w) {
        humpAt(w);
        // rings first, they spread around the hump and fade
        ctx.lineWidth = 1.2;
        for (let i = 0; i < w.ripples.length; i++) {
            const rp = w.ripples[i];
            const u = rp.t / RIPPLE_TIME;
            const r = w.R * lerp(0.5, 2.2, u);
            ctx.strokeStyle = 'rgba(255, 220, 170, ' + (0.12 * (1 - u)).toFixed(3) + ')';
            ctx.beginPath();
            ctx.ellipse(rp.x, g.ridgeAt(w.e, rp.x), r, r * 0.26, 0, 0, TWO_PI);
            ctx.stroke();
        }
        if (hump.h < 0.02) return;
        const pal = g.palette[w.e];
        const rx = 1.2 * w.R;
        const ry = 0.275 * w.R * hump.h;
        const cy = g.ridgeAt(w.e, hump.x) - 0.12 * w.R * hump.h;
        const grad = ctx.createLinearGradient(0, cy - ry, 0, cy + ry);
        grad.addColorStop(0, pal.crest);
        grad.addColorStop(1, pal.base);
        ctx.fillStyle = grad;
        ctx.beginPath();
        ctx.ellipse(hump.x, cy, rx, ry, 0, 0, TWO_PI);
        ctx.fill();
        // the flank toward the sun catches light
        ctx.strokeStyle = 'rgba(255, 214, 150, 0.22)';
        ctx.lineWidth = 1.5;
        ctx.beginPath();
        ctx.ellipse(hump.x, cy, rx, ry, 0, -Math.PI * 0.78, -Math.PI * 0.04);
        ctx.stroke();
    }

    function drawBody(ctx, g, w) {
        ctx.strokeStyle = 'rgba(24, 12, 6, 0.45)';
        ctx.lineWidth = 1.5;
        // Tail first, head last: every segment covers the one behind it, and the
        // outlines that stay visible are the rings of the body.
        for (let i = WORM_SEGMENTS; i >= 0; i--) {
            const s = w.sHead - i * WORM_STEP;
            if (s <= 0 || s >= 1) continue;
            wormPoint(g, w, s, P0);
            const r = i === 0 ? 1.15 * w.R : w.R * (1 - 0.6 * Math.pow(i / WORM_SEGMENTS, 1.4));
            if (w.sprite) {
                ctx.drawImage(w.sprite, P0.x - r, P0.y - r, 2 * r, 2 * r);
            } else {
                ctx.fillStyle = 'rgb(104, 66, 38)';
                ctx.beginPath();
                ctx.arc(P0.x, P0.y, r, 0, TWO_PI);
                ctx.fill();
            }
            ctx.beginPath();
            ctx.arc(P0.x, P0.y, r, 0, TWO_PI);
            ctx.stroke();
            if (i === 0) {
                // the maw opens while the head rises and closes before the apex
                const open = smoothstep(0.08, 0.28, w.sHead) * (1 - smoothstep(0.45, 0.62, w.sHead));
                if (open > 0.01) {
                    wormTangent(g, w, s, TAN);
                    drawMaw(ctx, w, P0.x, P0.y, r, open);
                }
            }
        }
    }

    // A dark mouth a little ahead of the head's centre, three pale teeth around its rim.
    function drawMaw(ctx, w, x, y, r, open) {
        const mx = x + TAN.x * 0.3 * r;
        const my = y + TAN.y * 0.3 * r;
        const m = 0.62 * r * open;
        ctx.fillStyle = '#140a05';
        ctx.beginPath();
        ctx.arc(mx, my, m, 0, TWO_PI);
        ctx.fill();
        const spin = Math.atan2(TAN.y, TAN.x) + w.age * 0.9;
        ctx.fillStyle = 'rgba(230, 200, 160, 0.55)';
        for (let k = 0; k < 3; k++) {
            const a = spin + k * TWO_PI / 3;
            ctx.beginPath();
            ctx.moveTo(mx + Math.cos(a - 0.3) * m, my + Math.sin(a - 0.3) * m);
            ctx.lineTo(mx + Math.cos(a) * m * 0.38, my + Math.sin(a) * m * 0.38);
            ctx.lineTo(mx + Math.cos(a + 0.3) * m, my + Math.sin(a + 0.3) * m);
            ctx.closePath();
            ctx.fill();
        }
        ctx.fillStyle = '#0a0503';
        ctx.beginPath();
        ctx.arc(mx, my, 0.28 * r * open, 0, TWO_PI);
        ctx.fill();
    }

    // Two patches of churned, darker sand where the worm went in and out.
    function drawDisturbed(ctx, g, w) {
        ctx.fillStyle = 'rgba(30, 18, 10, ' + (0.28 * (1 - w.t / SETTLE_TIME)).toFixed(3) + ')';
        const xs = [w.x0, w.x1];
        for (let k = 0; k < 2; k++) {
            ctx.beginPath();
            ctx.ellipse(xs[k], g.ridgeAt(w.e, xs[k]), 1.3 * w.R, 0.25 * w.R, 0, 0, TWO_PI);
            ctx.fill();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  HEAT SHIMMER
    //  A post-pass over the horizon band: thin horizontal strips of the
    //  finished scene are shifted sideways by a slow, layered wobble.
    // ═══════════════════════════════════════════════════════════════

    function updateShimmer(g) {
        const storm = g.stormIntensity;
        const amax = 2.2 * (1 - storm) * (1 - 0.5 * Math.min(1, g.gustStrength));
        shimmerAmax = storm < 0.6 && amax >= 0.3 ? amax : 0;
    }

    function ensureBand(width, rows) {
        if (!band) band = { canvas: document.createElement('canvas'), ctx: null };
        const cv = band.canvas;
        if (cv.width !== width || cv.height < rows) {
            cv.width = width;
            cv.height = rows + 48;           // headroom: the band moves a little with scroll and drift
            band.ctx = null;
        }
        if (!band.ctx) band.ctx = cv.getContext('2d');
        return band.ctx ? band : null;
    }

    function drawShimmer(ctx, g) {
        if (shimmerAmax <= 0) return;
        const cv = ctx.canvas;
        const dpr = g.dpr;
        const y0 = Math.max(0, g.layerTop(0) - 8);
        const y1 = Math.min(g.ridgeAt(1, 0) + 12, g.groundLine);
        const span = y1 - y0;
        if (span < 24) return;
        const devTop = Math.floor(y0 * dpr);
        const devBottom = Math.min(cv.height, Math.ceil(y1 * dpr));
        const rows = devBottom - devTop;
        const W = cv.width;
        if (rows < 8 || W < 8) return;
        const b = ensureBand(W, rows);
        if (!b) return;

        // Every strip reads from an untouched snapshot of the band (copying a
        // canvas onto itself strip by strip would copy it whole every time).
        b.ctx.setTransform(1, 0, 0, 1, 0, 0);
        b.ctx.clearRect(0, 0, W, rows);
        b.ctx.drawImage(cv, 0, devTop, W, rows, 0, 0, W, rows);

        ctx.save();
        try {
            // device pixels: strips sit on whole pixel rows, so neighbours never leave seams
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            ctx.globalAlpha = 1;
            ctx.globalCompositeOperation = 'source-over';
            const strip = Math.max(2, Math.round(3 * dpr));
            const t = g.weatherTime;
            for (let row = devTop; row < devBottom; row += strip) {
                const h = Math.min(strip, devBottom - row);
                const y = row / dpr;
                const amp = shimmerAmax * Math.sin(Math.PI * clamp((y - y0) / span, 0, 1));
                const off = amp * (Math.sin(y * 0.21 + t * 2.1) + 0.6 * Math.sin(y * 0.37 - t * 3.3 + 1.7)) * dpr;
                if (Math.abs(off) < 0.05) continue;
                // replace the strip (not paint over it) so soft edges move instead of doubling
                ctx.clearRect(0, row, W, h);
                ctx.drawImage(b.canvas, 0, row - devTop, W, h, off, row, W, h);
                // the sliver the shift uncovered is refilled with the pixels that left the other side
                if (off > 0) ctx.drawImage(b.canvas, W - off, row - devTop, off, h, 0, row, off, h);
                else ctx.drawImage(b.canvas, 0, row - devTop, -off, h, W + off, row, -off, h);
            }
        } finally {
            ctx.restore();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  CREST SAND WISPS
    //  In gusts the wind lifts sand off the dune crests: short streaks and
    //  faint banners stream downwind from the sharpest crests.
    // ═══════════════════════════════════════════════════════════════

    // x positions of the most prominent crests of one ridge (at most six).
    function findCrestXs(g, layer) {
        const step = 4;
        const n = Math.floor(g.width / step) + 1;
        const ys = new Float32Array(n);
        for (let k = 0; k < n; k++) ys[k] = g.ridgeAt(layer, k * step);
        const found = [];
        for (let k = 1; k < n - 1; k++) {
            if (!(ys[k] <= ys[k - 1] && ys[k] < ys[k + 1])) continue;       // a local peak of the ridge
            // prominence: how far the ridge drops on both sides before it climbs above this crest again
            let left = ys[k], right = ys[k];
            for (let j = k - 1; j >= 0 && ys[j] >= ys[k]; j--) left = Math.max(left, ys[j]);
            for (let j = k + 1; j < n && ys[j] >= ys[k]; j++) right = Math.max(right, ys[j]);
            const prominence = Math.min(left, right) - ys[k];
            if (prominence >= 8) found.push({ x: k * step, prominence: prominence });
        }
        found.sort(function (a, b) { return b.prominence - a.prominence; });
        found.length = Math.min(found.length, 6);
        found.sort(function (a, b) { return a.x - b.x; });
        return found.map(function (c) { return c.x; });
    }

    // The ridge drifts slowly, so the crests are looked up again every 2 s.
    function scanCrests(g) {
        crestStamp = g.weatherTime;
        crestList = [];
        for (let layer = 0; layer < 2; layer++) {
            const xs = findCrestXs(g, layer);
            for (let k = 0; k < xs.length; k++) crestList.push({ layer: layer, x: xs[k], carry: 0 });
        }
    }

    function spawnWisp(g, c) {
        const p = takeFree(wisps);
        if (!p) return false;                        // full: let the living ones finish their streak
        p.on = true;
        p.layer = c.layer;
        p.x = c.x + rand(-6, 6);
        p.y = g.ridgeAt(c.layer, c.x) - 1;
        p.vx = g.windDirection * (35 + 25 * windExcess + rand(0, 20));
        p.vy = -8 + rand(-6, 4);
        p.t = 0;
        p.life = rand(0.7, 1.3);
        return true;
    }

    function updateWisps(g) {
        if (g.layers < 2) return;
        windExcess = Math.max(0, g.windSpeed - g.baseWind - 0.5) + 2.5 * g.gustStrength;
        if (g.weatherTime - crestStamp >= 2 || g.weatherTime < crestStamp) scanCrests(g);

        for (let i = 0; i < wisps.items.length; i++) {
            const p = wisps.items[i];
            if (!p.on) continue;
            p.t += sdt;
            if (p.t >= p.life) { p.on = false; continue; }
            p.x += p.vx * sdt;
            p.y += p.vy * sdt + layerShift[p.layer];
        }

        const n = crestList.length;
        if (windExcess <= 0.01 || n === 0) return;
        for (let k = 0; k < n; k++) {
            const c = crestList[(k + spawnRotor) % n];
            // The engine's calm wind already gusts to windExcess ~0.65 every few seconds: at this
            // rate that is 20-60 streaks in the air, while a storm (excess ~6) still fills the pool.
            c.carry = Math.min(c.carry + windExcess * 0.12 * dtFrames, 3);
            while (c.carry >= 1) {
                if (!spawnWisp(g, c)) { c.carry = 0; break; }
                c.carry -= 1;
            }
        }
        spawnRotor = (spawnRotor + 1) % n;           // so no crest starves when the pool is nearly full
    }

    function drawWisps(ctx, g, layer) {
        if (g.layers < 2) return;
        drawFlags(ctx, g, layer);
        ctx.lineWidth = 1.2;
        for (let i = 0; i < wisps.items.length; i++) {
            const p = wisps.items[i];
            if (!p.on || p.layer !== layer) continue;
            const a = 0.28 * (1 - p.t / p.life);
            if (a < 0.01) continue;
            ctx.strokeStyle = 'rgba(236, 208, 160, ' + a.toFixed(3) + ')';
            ctx.beginPath();
            ctx.moveTo(p.x, p.y);
            ctx.lineTo(p.x - p.vx * 0.08, p.y - p.vy * 0.08);
            ctx.stroke();
        }
    }

    // In strong gusts every crest also flies a faint banner of sand.
    function drawFlags(ctx, g, layer) {
        if (windExcess <= 0.3 || Math.abs(g.windDirection) < 0.08) return;
        const dir = g.windDirection > 0 ? 1 : -1;
        const k = Math.min(1, windExcess);
        const len = 24 + 40 * k;
        const thick = 3 + 5 * k;
        ctx.fillStyle = 'rgba(236, 208, 160, ' + (0.05 + 0.07 * k).toFixed(3) + ')';
        for (let i = 0; i < crestList.length; i++) {
            const c = crestList[i];
            if (c.layer !== layer) continue;
            const y = g.ridgeAt(layer, c.x) - 1;
            const wave = Math.sin(g.weatherTime * 3 + c.x);
            const wave2 = Math.sin(g.weatherTime * 3 + c.x + 1.7);
            ctx.beginPath();
            ctx.moveTo(c.x, y);
            ctx.quadraticCurveTo(c.x + dir * len * 0.45, y - thick * 1.5 + wave * 2.2,
                c.x + dir * len, y - thick * 0.2 + wave * 3);
            ctx.quadraticCurveTo(c.x + dir * len * 0.5, y - thick * 0.15 + wave2 * 1.6, c.x, y);
            ctx.closePath();
            ctx.fill();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  VULTURES (stage -1, in the sky)
    //  Two birds over a narrow chat, three over a wide one, ride a thermal in the upper
    //  sky, each on its own slow anti-clockwise ellipse. The thermal starts beside the
    //  sun, but the chat bubbles hide whatever is behind them: before the birds glide in,
    //  and every VULTURE_RELOCATE seconds while they circle, it looks for the spot where
    //  the fewest of them would be covered, whichever way the thermal sways (pickThermal),
    //  and drifts there. After a start they glide in from the nearest screen edge, a storm
    //  sends them off with the dust wall and they come back a while after it. Their places
    //  are recomputed from the live geometry every frame, so a resize needs no work: the
    //  next update re-centres them.
    // ═══════════════════════════════════════════════════════════════

    const birds = [];
    const orbit = { x: 0, y: 0 };                 // scratch: where the thermal puts a bird right now
    // The thermal: x, y is where the flock circles (before the sway), tx, ty where it is heading, due
    // the seconds in view until the next look at the chat (0: on the next update) and picked whether
    // tx, ty was chosen since the last start or resize.
    const thermal = { x: 0, y: 0, tx: 0, ty: 0, due: 0, picked: false };
    const bounds = { x0: 0, x1: 0, y0: 0, y1: 0 };   // scratch: where the centre of the ellipses may be

    function makeBird() {
        return {
            state: 'off',          // off (due to glide in), enter, orbit, flee, gone (waiting to come back)
            alpha: 0,
            x: 0, y: 0,            // where it is drawn
            ang: 0,                // place on its ellipse; counts down, which is anti-clockwise on screen
            period: 20, size: 7, sway: 0,
            enterT: 0, edge: 1,    // glide-in progress 0..1 and the side it comes from
            fdir: 1, fvx: 0,       // flight from a storm
            returnIn: -1,          // seconds until it comes back, -1 while that clock is not running
            flapT: -1, flapIn: 8   // flap burst: elapsed (-1 = gliding) and seconds to the next one
        };
    }

    for (let i = 0; i < VULTURE_SLOTS; i++) birds.push(makeBird());

    function clearFlock() {
        for (let j = 0; j < birds.length; j++) {
            const b = birds[j];
            b.state = 'off';
            b.alpha = 0;
            b.returnIn = -1;
            b.flapT = -1;
            b.fvx = 0;
            b.enterT = 0;
        }
    }

    // A new run: every bird gets its own ellipse speed, size and starting point. The size is half
    // the wing span in px; below 6 a bird is hard to make out at 1x.
    function rollFlock() {
        clearFlock();
        for (let j = 0; j < birds.length; j++) {
            const b = birds[j];
            b.ang = rand(0, TWO_PI);
            b.period = rand(16, 24);
            b.size = rand(6, 10);
            b.sway = rand(0, TWO_PI);
            b.flapIn = rand(3, 11);
        }
    }

    function vulturesWanted(g) {
        return g.width > 1100 ? 3 : 2;
    }

    // Bird j has the j-th ellipse: rx wide and 0.3 rx high.
    function thermalRx(g, j) {
        return (0.07 + 0.015 * j) * g.width;
    }

    // The whole thermal sways a little to and fro, THERMAL_SWAY of the width either way.
    function thermalSway(g) {
        return THERMAL_SWAY * g.width * Math.sin(g.weatherTime * 0.07);
    }

    // The thermal is where pickThermal and driftThermal put it (thermal.x, thermal.y), plus the sway.
    function placeOnThermal(g, b, j) {
        const rx = thermalRx(g, j);
        orbit.x = thermal.x + thermalSway(g) + rx * Math.cos(b.ang);
        orbit.y = thermal.y + 0.3 * rx * Math.sin(b.ang);
    }

    // Where the centre of the ellipses may be: the biggest one, with the sway on top of it, stays 8 px
    // inside the canvas and the whole flock inside the sky band 0.12 h .. 0.6 h. Fills `bounds`.
    function thermalBounds(g, want) {
        const rxMax = thermalRx(g, want - 1);
        const edge = rxMax + 8 + THERMAL_SWAY * g.width;
        bounds.x0 = edge;
        bounds.x1 = g.width - edge;
        bounds.y0 = 0.12 * g.height + 0.3 * rxMax;
        bounds.y1 = 0.6 * g.height - 0.3 * rxMax;
    }

    // How many of the wanted birds are over the thermal (gliding in or circling).
    function birdsOnThermal(want) {
        let n = 0;
        for (let j = 0; j < want; j++) {
            const s = birds[j].state;
            if (s === 'enter' || s === 'orbit') n++;
        }
        return n;
    }

    // How much of the flock would sit behind chat bubbles with the thermal centred on (cx, cy): of
    // THERMAL_SAMPLES points on each wanted ellipse the hidden ones are counted, and the shares add up
    // over the ellipses. The points include the right-most (0 degrees) and the left-most (180 degrees)
    // of the ellipse, where a bird comes closest to a bubble edge. The sway carries the flock to either
    // side while it circles, so the count is made twice, with the centre pushed to the left end and to
    // the right end of the sway, and the worse of the two counts: a spot that is clear now but under a
    // bubble edge 20 s later is not in the clear. 0 is all in the clear, `want` all hidden. Uses the
    // rectangles `occ` holds.
    function thermalHidden(g, want, cx, cy) {
        const reach = THERMAL_SWAY * g.width;
        let hiddenLeft = 0, hiddenRight = 0;
        for (let j = 0; j < want; j++) {
            const rx = thermalRx(g, j);
            for (let k = 0; k < THERMAL_SAMPLES; k++) {
                const a = k * TWO_PI / THERMAL_SAMPLES;
                const dx = rx * Math.cos(a), y = cy + 0.3 * rx * Math.sin(a);
                if (isHidden(cx - reach + dx, y)) hiddenLeft++;
                if (isHidden(cx + reach + dx, y)) hiddenRight++;
            }
        }
        return Math.max(hiddenLeft, hiddenRight) / THERMAL_SAMPLES;
    }

    // Reads the chat and sets the target of the thermal to the spot where the fewest birds would sit behind
    // a bubble (thermalHidden scores both ends of the sway, lowest wins). Candidates, in this order: the
    // current target, the default spot left of the sun, and THERMAL_TRIES random centres in the upper
    // sky. The random ones carry THERMAL_BIAS, so the flock only moves for a real gain, and a known spot
    // that is wholly in the clear ends the search. With no rectangles to read (no canvas, no chat)
    // everything is in the clear and the current target, or else the default spot, stays. This is the
    // only place the vultures read the DOM.
    function pickThermal(g) {
        const want = vulturesWanted(g);
        collectOccluders(g, 0.12 * g.height, 0.58 * g.height);
        thermalBounds(g, want);
        const rxMax = thermalRx(g, want - 1);
        let best = Infinity, bx = 0, by = 0;
        for (let c = thermal.picked ? 0 : 1; c < 2 + THERMAL_TRIES; c++) {
            let cx, cy, bias = 0;
            if (c === 0) {
                cx = thermal.tx;
                cy = thermal.ty;
            } else if (c === 1) {
                cx = g.sunX - 0.16 * g.width;
                cy = g.sunY + 0.19 * g.height;
            } else {
                cx = rand(rxMax + 24, g.width - rxMax - 24);
                cy = rand(0.22 * g.height, 0.42 * g.height);
                bias = THERMAL_BIAS;
            }
            cx = clamp(cx, bounds.x0, bounds.x1);
            cy = clamp(cy, bounds.y0, bounds.y1);
            const score = thermalHidden(g, want, cx, cy) + bias;
            if (score < best) {
                best = score;
                bx = cx;
                by = cy;
            }
            if (c < 2 && score === 0) break;
        }
        thermal.tx = bx;
        thermal.ty = by;
        thermal.picked = true;
    }

    // Nobody is up there and a bird is about to glide in: choose the spot and start on it.
    function settleThermal(g) {
        pickThermal(g);
        thermal.x = thermal.tx;
        thermal.y = thermal.ty;
        thermal.due = VULTURE_RELOCATE;
    }

    // Birds are over the thermal: every VULTURE_RELOCATE seconds look at the chat again, and let the
    // thermal (the flock with it) drift toward the target, at most THERMAL_DRIFT px/s and slower in the
    // last few px. The centre also stays inside the bounds of the live geometry, which a resize moved.
    function driftThermal(g, want) {
        thermal.due -= sdt;
        if (thermal.due <= 0) {
            pickThermal(g);
            thermal.due = VULTURE_RELOCATE;
        }
        const dx = thermal.tx - thermal.x, dy = thermal.ty - thermal.y;
        const d = Math.sqrt(dx * dx + dy * dy);
        if (d < 0.25) {
            thermal.x = thermal.tx;
            thermal.y = thermal.ty;
        } else {
            const k = Math.min(THERMAL_DRIFT, d * 1.5) * sdt / d;
            thermal.x += dx * k;
            thermal.y += dy * k;
        }
        thermalBounds(g, want);
        thermal.x = clamp(thermal.x, bounds.x0, bounds.x1);
        thermal.y = clamp(thermal.y, bounds.y0, bounds.y1);
    }

    function glideIn(g, b, j) {
        placeOnThermal(g, b, j);
        b.state = 'enter';
        b.enterT = 0;
        b.edge = orbit.x < g.width / 2 ? -1 : 1;       // the nearest side
        b.alpha = 0;
    }

    function countVultures() {
        let n = 0;
        for (let j = 0; j < birds.length; j++) if (birds[j].alpha > 0.01) n++;
        return n;
    }

    function updateVultures(g) {
        const want = vulturesWanted(g);
        const storm = g.stormIntensity;
        const stormy = g.stormActive || storm > 0;
        if (birdsOnThermal(want) > 0) driftThermal(g, want);
        for (let j = 0; j < birds.length; j++) {
            const b = birds[j];
            b.ang -= TWO_PI * sdt / b.period;
            if (b.ang < 0) b.ang += TWO_PI;
            if (j >= want) {                          // the chat got narrower: the third bird is not wanted
                b.state = 'off';
                b.alpha = 0;
                continue;
            }
            if (b.state === 'off') {
                if (stormy) {                                            // wait the storm out of sight
                    b.state = 'gone';
                    b.returnIn = -1;
                } else {
                    if (birdsOnThermal(want) === 0) settleThermal(g);    // the first one up chooses the spot
                    glideIn(g, b, j);
                }
            }
            if ((b.state === 'enter' || b.state === 'orbit') && storm > VULTURE_FLEE_AT) {
                b.state = 'flee';
                b.fdir = g.stormDirection >= 0 ? 1 : -1;    // with the wall, away from the side it enters from
                b.fvx = 0;
            }
            if (b.state === 'enter') {
                b.enterT = Math.min(1, b.enterT + sdt / VULTURE_GLIDE_IN);
                placeOnThermal(g, b, j);
                const k = smoothstep(0, 1, b.enterT);
                b.x = lerp(b.edge < 0 ? -24 : g.width + 24, orbit.x, k);
                b.y = lerp(orbit.y - 14, orbit.y, k);
                b.alpha = k;
                if (b.enterT >= 1) b.state = 'orbit';
            } else if (b.state === 'orbit') {
                placeOnThermal(g, b, j);
                b.x = orbit.x;
                b.y = orbit.y;
                b.alpha = 1;
            } else if (b.state === 'flee') {
                b.fvx = clamp(b.fvx + b.fdir * VULTURE_FLEE_PUSH * sdt, -VULTURE_FLEE_SPEED, VULTURE_FLEE_SPEED);
                b.x += b.fvx * sdt;
                b.y -= 14 * sdt;
                if (b.x < -30 || b.x > g.width + 30) {   // past the edge: invisible until they come back
                    b.state = 'gone';
                    b.alpha = 0;
                    b.returnIn = -1;
                }
            } else if (b.state === 'gone') {
                if (stormy) b.returnIn = -1;              // the clock starts when the dust has settled
                else if (b.returnIn < 0) b.returnIn = rand(VULTURE_RETURN_MIN, VULTURE_RETURN_MAX);
                else if ((b.returnIn -= sdt) <= 0) { b.state = 'off'; b.returnIn = -1; }
            }
            if (b.state === 'enter' || b.state === 'orbit' || b.state === 'flee') {
                if (b.flapT >= 0) {
                    b.flapT += sdt;
                    if (b.flapT >= FLAP_TIME) { b.flapT = -1; b.flapIn = rand(6, 11); }
                } else if ((b.flapIn -= sdt) <= 0) {
                    b.flapT = 0;
                }
            }
        }
    }

    // Two quadratic wings with a dihedral, a tiny body between them.
    function drawBird(ctx, g, b) {
        // The wings fold toward us when the bird flies across the view, and the inner wing dips in the turn.
        const c = Math.cos(b.ang);
        const span = b.size * (0.62 + 0.38 * Math.abs(c));
        const dip = 1.1 * c;
        let lift;                                     // wing-tip travel, positive = up
        if (b.state === 'flee') {
            lift = Math.sin(g.weatherTime * 13 + b.sway) * 3;                  // hurried beats
        } else if (b.flapT >= 0) {
            const u = b.flapT / FLAP_TIME;
            lift = Math.sin(u * 3 * TWO_PI) * 3 * Math.min(1, u / 0.1, (1 - u) / 0.1);
        } else {
            lift = Math.sin(g.weatherTime * 0.9 + b.sway) * 0.6;               // a slow sway
        }
        const x = b.x, y = b.y;
        // Each wing is a leading edge out to the tip and a trailing edge back: a hairline
        // would vanish against the sky at this size, a tapered wing keeps its weight at the root.
        const tipL = y - 2 - lift + dip, tipR = y - 2 - lift - dip;
        ctx.beginPath();
        ctx.moveTo(x, y - 0.6);
        ctx.quadraticCurveTo(x - span * 0.5, y - 0.9, x - span, tipL);
        ctx.quadraticCurveTo(x - span * 0.48, y + 1.5, x, y + 1);
        ctx.moveTo(x, y - 0.6);
        ctx.quadraticCurveTo(x + span * 0.5, y - 0.9, x + span, tipR);
        ctx.quadraticCurveTo(x + span * 0.48, y + 1.5, x, y + 1);
        ctx.fill();
        ctx.stroke();
        ctx.beginPath();
        ctx.ellipse(x, y + 0.2, 1.7, 1, 0, 0, TWO_PI);
        ctx.fill();
    }

    function drawVultures(ctx, g, fade) {
        ctx.save();
        try {
            ctx.strokeStyle = 'rgb(28, 16, 8)';
            ctx.fillStyle = 'rgb(28, 16, 8)';
            ctx.lineWidth = 1;
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            for (let j = 0; j < birds.length; j++) {
                const b = birds[j];
                if (b.alpha <= 0.01) continue;
                ctx.globalAlpha = 0.85 * b.alpha * fade;
                drawBird(ctx, g, b);
            }
        } finally {
            ctx.restore();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  TUMBLEWEED (stage 1, mid ridge)
    //  A ball of dry twigs enters from the upwind edge, hops along the ridge at the
    //  pace of the wind and leaves 40 px past the far edge. A storm throws one more as
    //  it breaks, but no scheduled ball starts while it lasts: that one waits, 2 s at a
    //  time (the worm, the beetle and the caravan keep out of storms too).
    //  The direction is fixed when it enters (the wind direction swings through zero
    //  every 20 s or so, and a ball that stood still there would never cross).
    // ═══════════════════════════════════════════════════════════════

    const weeds = [];
    for (let i = 0; i < MAX_TUMBLEWEEDS; i++) {
        weeds.push({
            on: false, x: 0, y: 0, vx: 0, vy: 0, ang: 0, r: 12, dir: 1, hopIn: 0, grounded: true,
            sprite: null, sctx: null, half: 14
        });
    }

    function liveWeeds() {
        let n = 0;
        for (let i = 0; i < weeds.length; i++) if (weeds[i].on) n++;
        return n;
    }

    // ~16 random cubic curves inside the circle, 8 short spikes on the rim.
    function paintWeed(w, dpr) {
        const half = Math.ceil(w.r * 1.3) + 2;                 // CSS px from the centre to the sprite edge
        const px = Math.max(8, Math.ceil(half * 2 * dpr));
        if (!w.sprite) {
            w.sprite = makeCanvas(px, px);
            w.sctx = w.sprite.getContext('2d');
        } else if (w.sprite.width !== px) {
            w.sprite.width = px;                               // resizing clears it
            w.sprite.height = px;
        }
        const c = w.sctx;
        if (!c) { w.sprite = null; return; }
        w.half = half;
        const r = w.r;
        c.setTransform(1, 0, 0, 1, 0, 0);
        c.clearRect(0, 0, px, px);
        c.setTransform(dpr, 0, 0, dpr, half * dpr, half * dpr);   // origin in the middle, CSS px
        c.lineCap = 'round';
        c.save();
        c.beginPath();
        c.arc(0, 0, r, 0, TWO_PI);
        c.clip();
        const core = c.createRadialGradient(0, 0, 0, 0, 0, r);   // a soft dark core, so the twigs read as a ball
        core.addColorStop(0, 'rgba(48, 29, 14, 0.5)');
        core.addColorStop(0.7, 'rgba(48, 29, 14, 0.28)');
        core.addColorStop(1, 'rgba(48, 29, 14, 0)');
        c.fillStyle = core;
        c.fill();
        // Every twig gets a faint dark edge under its tan: the tan alone melts into the hazy far dune.
        for (let k = 0; k < 16; k++) {
            const width = rand(1, 1.4);
            const a0 = rand(0, TWO_PI);
            const a1 = a0 + rand(1.6, 4.6);                    // the twig crosses the ball
            c.beginPath();
            c.moveTo(Math.cos(a0) * r * rand(0.82, 1), Math.sin(a0) * r * rand(0.82, 1));
            c.bezierCurveTo(rand(-0.8, 0.8) * r, rand(-0.8, 0.8) * r, rand(-0.8, 0.8) * r, rand(-0.8, 0.8) * r,
                Math.cos(a1) * r * rand(0.82, 1), Math.sin(a1) * r * rand(0.82, 1));
            c.lineWidth = width + 1.4;
            c.strokeStyle = 'rgba(40, 24, 12, 0.3)';
            c.stroke();
            c.lineWidth = width;
            c.strokeStyle = 'rgba(150, 118, 70, 0.9)';
            c.stroke();
        }
        c.restore();
        for (let k = 0; k < 8; k++) {
            const a = (k + rand(-0.25, 0.25)) * TWO_PI / 8;
            const inner = r * rand(0.84, 0.95);
            const outer = r * rand(1.12, 1.28);
            c.beginPath();
            c.moveTo(Math.cos(a) * inner, Math.sin(a) * inner);
            c.lineTo(Math.cos(a) * outer, Math.sin(a) * outer);
            c.lineWidth = 2.4;
            c.strokeStyle = 'rgba(40, 24, 12, 0.3)';
            c.stroke();
            c.lineWidth = 1;
            c.strokeStyle = 'rgba(150, 118, 70, 0.9)';
            c.stroke();
        }
    }

    // From the upwind edge, or at opts.x; false when both balls are already rolling.
    function spawnWeed(g, opts) {
        let w = null;
        for (let i = 0; i < weeds.length; i++) {
            if (!weeds[i].on) { w = weeds[i]; break; }
        }
        if (!w) return false;
        const o = opts || {};
        w.on = true;
        w.dir = o.dir < 0 ? -1 : (o.dir > 0 ? 1 : (g.windDirection >= 0 ? 1 : -1));
        w.r = rand(11, 16);
        w.x = typeof o.x === 'number' && isFinite(o.x) ? o.x : (w.dir > 0 ? -30 : g.width + 30);
        w.vx = w.dir * (45 + 22 * g.windSpeed);
        w.vy = 0;
        w.y = g.ridgeAt(1, w.x) - w.r;
        w.ang = rand(0, TWO_PI);
        w.hopIn = rand(0.3, 0.9);
        w.grounded = true;
        paintWeed(w, g.dpr);
        return true;
    }

    function updateTumbleweeds(g, time) {
        if (g.layers < 2) return;
        if (g.stormActive && !prevStormActive && Math.random() < 0.6) spawnWeed(g, null);   // a storm throws one more
        if (time >= nextTumbleTime) {
            if (g.stormActive || g.stormIntensity > 0) nextTumbleTime = time + 2000;   // storming: the scheduled ball waits
            else nextTumbleTime = spawnWeed(g, null) ? nextTumbleAfter(time) : time + 2000;
        }
        while (requests.tumbleweed.length > 0) {
            if (!spawnWeed(g, requests.tumbleweed.shift())) { requests.tumbleweed.length = 0; break; }
        }
        for (let i = 0; i < weeds.length; i++) {
            const w = weeds[i];
            if (!w.on) continue;
            // the wind sets the pace; the ball eases into it
            w.vx += (w.dir * (45 + 22 * g.windSpeed) - w.vx) * Math.min(1, 3 * sdt);
            w.x += w.vx * sdt;
            w.ang += w.vx / w.r * sdt;
            w.hopIn -= sdt;
            if (!w.grounded) w.y += layerShift[1];       // in the air the dune can slide away under the ball
            if (w.grounded && w.hopIn <= 0) {
                w.vy = -rand(60, 140);
                w.hopIn = rand(0.5, 1.1);
                w.grounded = false;
            }
            w.vy += WEED_GRAVITY * sdt;
            w.y += w.vy * sdt;
            const floor = g.ridgeAt(1, w.x) - w.r;
            if (w.vy >= 0 && w.y >= floor - 1.5) {         // on the ridge (rolling downhill stays glued to it)
                w.y = floor;
                w.vy = 0;
                w.grounded = true;
            } else {
                w.grounded = false;
            }
            if (w.dir > 0 ? w.x > g.width + 40 : w.x < -40) w.on = false;
        }
    }

    function drawTumbleweeds(ctx, g, fade) {
        if (g.layers < 2) return;
        ctx.save();
        try {
            ctx.fillStyle = 'rgb(20, 10, 5)';
            for (let i = 0; i < weeds.length; i++) {
                const w = weeds[i];
                if (!w.on || w.x < -w.half || w.x > g.width + w.half) continue;
                const ridge = g.ridgeAt(1, w.x);
                const up = Math.max(0, ridge - w.r - w.y);                   // height above the ground
                const sx = 0.8 * w.r / (1 + up / (2.5 * w.r));               // the shadow shrinks as the ball lifts
                const sy = sx * 0.24 + 0.1;
                ctx.globalAlpha = 0.22 * fade;
                ctx.beginPath();
                ctx.ellipse(w.x, ridge + sy * 0.9, sx, sy, 0, 0, TWO_PI);
                ctx.fill();
                ctx.globalAlpha = fade;
                ctx.save();
                try {
                    ctx.translate(w.x, w.y);
                    ctx.rotate(w.ang);
                    if (w.sprite) {
                        ctx.drawImage(w.sprite, -w.half, -w.half, w.half * 2, w.half * 2);
                    } else {
                        ctx.strokeStyle = 'rgba(150, 118, 70, 0.9)';
                        ctx.lineWidth = 1.2;
                        ctx.beginPath();
                        ctx.arc(0, 0, w.r, 0, TWO_PI);
                        ctx.stroke();
                    }
                } finally {
                    ctx.restore();
                }
            }
        } finally {
            ctx.restore();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  BEETLE (stage 1, mid ridge, calm only)
    //  Digs out of the sand, walks in short bursts with pauses, and after 20-35 s
    //  (or at the first breath of a storm) burrows in again.
    // ═══════════════════════════════════════════════════════════════

    const beetle = {
        on: false, state: 'rise',   // rise (digging out), pause, walk, sink (burrowing)
        x: 0, dir: 1, t: 0, left: 0, life: 0, sink: 0, speed: 30, gait: 0, trickle: 0
    };
    const BEETLE_LEG_X = [2.4, 0.4, -2.0];     // where the leg pairs hang under the body

    // A beetle sits still for most of its 20-35 s, so it is worth picking a spot the chat
    // leaves free and the near dune does not swallow. Six tries, fewest hidden samples win.
    function pickBeetleX(g, lo, hi) {
        collectOccluders(g, g.groundLine - 150, g.groundLine);
        let best = rand(lo, hi), bestRank = Infinity;
        for (let k = 0; k < 6; k++) {
            const x = k === 0 ? best : rand(lo, hi);
            let rank = wormSpotVisible(g, 1, x, x) ? 0 : 10;
            if (occ.n) {
                for (let s = -1; s <= 1; s++) {
                    if (isHidden(x + s * 40, g.ridgeAt(1, x + s * 40) - 3)) rank += 1;
                }
            }
            if (rank < bestRank) { bestRank = rank; best = x; }
            if (rank === 0) break;
        }
        return best;
    }

    function spawnBeetle(g, opts) {
        const o = opts || {};
        const b = beetle;
        const lo = 0.06 * g.width, hi = 0.94 * g.width;
        b.on = true;
        b.x = typeof o.x === 'number' && isFinite(o.x) ? clamp(o.x, lo, hi) : pickBeetleX(g, lo, hi);
        b.dir = o.dir < 0 ? -1 : (o.dir > 0 ? 1 : (Math.random() < 0.5 ? -1 : 1));
        b.state = 'rise';
        b.t = 0;
        b.left = 0;
        b.life = rand(20, 35);
        b.sink = BEETLE_DEPTH;
        b.speed = rand(22, 38);
        b.gait = 0;
        b.trickle = 0;
    }

    // Six grains trickle up while the beetle digs in or out.
    function digSand(g, b) {
        b.trickle += 6 / BEETLE_DIG * sdt;
        while (b.trickle >= 1) {
            b.trickle -= 1;
            emitSpray(1, b.x + rand(-2.5, 2.5), g.ridgeAt(1, b.x) - 0.5, rand(-14, 14), rand(-34, -10), rand(0.3, 0.6), rand(0.8, 1.4));
        }
    }

    function updateBeetle(g, time) {
        if (g.layers < 2) return;
        const b = beetle;
        if (!b.on) {
            if (requests.beetle) {
                const request = requests.beetle;
                requests.beetle = null;
                spawnBeetle(g, request);
            } else if (time >= nextBeetleTime) {
                if (calmFor(g, CRITTER_CALM_MS)) {
                    spawnBeetle(g, null);
                    nextBeetleTime = nextBeetleAfter(time);
                } else {
                    nextBeetleTime = time + 2000;
                }
            }
            if (!b.on) return;
        }
        b.t += sdt;
        if (b.state !== 'sink') {
            if (b.state !== 'rise') b.life -= sdt;
            if (b.life <= 0 || g.stormIntensity > 0.15) {      // time is up, or a storm is coming: burrow
                b.state = 'sink';
                b.t = BEETLE_DIG * (b.sink / BEETLE_DEPTH);    // from where it is: a half-risen beetle does not pop up first
                b.trickle = 0;
            }
        }
        if (b.state === 'rise') {
            b.sink = BEETLE_DEPTH * (1 - smoothstep(0, 1, b.t / BEETLE_DIG));
            digSand(g, b);
            if (b.t >= BEETLE_DIG) { b.state = 'pause'; b.t = 0; b.left = rand(0.4, 1.0); }
        } else if (b.state === 'pause') {
            if (b.t >= b.left) {                                // a new burst, now and then the other way
                b.state = 'walk';
                b.t = 0;
                b.left = rand(0.6, 1.5);
                b.speed = rand(22, 38);
                if (Math.random() < 0.2) b.dir = -b.dir;
            }
        } else if (b.state === 'walk') {
            const lo = 0.06 * g.width, hi = 0.94 * g.width;
            b.x += b.dir * b.speed * sdt;
            if (b.x < lo) { b.x = lo; b.dir = 1; } else if (b.x > hi) { b.x = hi; b.dir = -1; }
            b.gait += sdt;
            if (b.t >= b.left) { b.state = 'pause'; b.t = 0; b.left = rand(0.4, 1.8); }
        } else {
            b.sink = BEETLE_DEPTH * smoothstep(0, 1, b.t / BEETLE_DIG);
            digSand(g, b);
            if (b.t >= BEETLE_DIG) b.on = false;
        }
    }

    // Local frame: origin on the ridge under the beetle, +x forward, -y up.
    function traceBeetle(ctx, b) {
        const walking = b.state === 'walk';
        const set = walking ? Math.floor(b.gait / BEETLE_GAIT) & 1 : 0;     // which tripod is on the move
        ctx.strokeStyle = '#1c110a';
        ctx.lineWidth = 1;
        ctx.lineCap = 'round';
        ctx.beginPath();
        for (let i = 0; i < 3; i++) {
            for (let side = 0; side < 2; side++) {
                const tripod = (i + side) & 1;                  // front near, middle far, hind near | the others
                const swing = walking ? (tripod === set ? 1 : -1) : 0;
                const ax = BEETLE_LEG_X[i] + side * 0.5;
                ctx.moveTo(ax, -2);
                ctx.lineTo(ax + swing * 1.1, walking && tripod === set ? -0.7 : 0);
            }
        }
        ctx.stroke();
        ctx.fillStyle = '#1c110a';
        ctx.beginPath();
        ctx.moveTo(1.15, -2.6);
        ctx.ellipse(-1.6, -2.6, 2.75, 1.9, 0, 0, TWO_PI);       // abdomen, 5.5 px
        ctx.moveTo(3.4, -2.5);
        ctx.ellipse(1.9, -2.5, 1.5, 1.2, 0, 0, TWO_PI);         // thorax, 3 px
        ctx.moveTo(4.6, -2.3);
        ctx.arc(3.75, -2.3, 0.85, 0, TWO_PI);                   // head
        ctx.fill();
        ctx.strokeStyle = 'rgba(120, 90, 60, 0.5)';              // a glint along the back
        ctx.lineWidth = 0.7;
        ctx.beginPath();
        ctx.moveTo(-3.2, -3.7);
        ctx.quadraticCurveTo(-1.6, -4.8, 0.3, -4.2);
        ctx.stroke();
    }

    function drawBeetle(ctx, g, fade) {
        const b = beetle;
        if (!b.on || g.layers < 2) return;
        const x = b.x;
        const ridge = g.ridgeAt(1, x);
        const tilt = Math.atan2(g.ridgeAt(1, x + 4) - g.ridgeAt(1, x - 4), 8);   // follow the slope
        ctx.save();
        try {
            if (b.sink > 0.05) {                 // the buried part is cut off by the sand
                ctx.beginPath();
                ctx.moveTo(x - 12, g.ridgeAt(1, x - 12));
                ctx.lineTo(x - 6, g.ridgeAt(1, x - 6));
                ctx.lineTo(x, ridge);
                ctx.lineTo(x + 6, g.ridgeAt(1, x + 6));
                ctx.lineTo(x + 12, g.ridgeAt(1, x + 12));
                ctx.lineTo(x + 12, ridge - 30);
                ctx.lineTo(x - 12, ridge - 30);
                ctx.closePath();
                ctx.clip();
            }
            ctx.globalAlpha = fade;
            ctx.translate(x, ridge + b.sink);
            ctx.rotate(tilt);
            ctx.scale(b.dir, 1);
            traceBeetle(ctx, b);
        } finally {
            ctx.restore();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  CARAVAN (stage 0, far ridge, calm only)
    //  4-6 camels, every other one with a rider, walk in a line along the far ridge.
    //  The calm stretches of this scene are short (12-24 s) and a camel does 11-15
    //  px/s, so the line fades in out of the haze somewhere on screen instead of
    //  walking in from an edge; a storm dissolves it in 2 s.
    // ═══════════════════════════════════════════════════════════════

    const caravan = { on: false, dir: 1, speed: 13, lead: 0, tail: 0, n: 0, age: 0, leave: -1, camels: [] };
    for (let i = 0; i < MAX_CAMELS; i++) {
        caravan.camels.push({ off: 0, h: 10, phase: 0, omega: 8, rider: false });
    }
    const CAMEL_LEG_X = [-0.36, -0.2, 0.2, 0.36];     // leg positions along the body, in camel heights

    // The leader's x, the whole line on screen: the far ridge must stand clear of the mid dune
    // along the stretch the line will cover, the chat should leave it free, and there should be
    // some room to walk. Ten tries, the lowest rank wins and the earliest on a tie.
    function pickCaravanLead(g, c) {
        const w = g.width;
        let lo = c.dir > 0 ? c.tail + 24 : 24;
        let hi = c.dir > 0 ? w - 24 : w - c.tail - 24;
        if (hi < lo) lo = hi = 0.5 * w;
        collectOccluders(g, g.groundLine - 220, g.groundLine - 20);
        let best = rand(lo, hi), bestRank = Infinity;
        for (let k = 0; k < 10; k++) {
            const x = k === 0 ? best : rand(lo, hi);
            const from = x - c.dir * c.tail, to = x + c.dir * 150;     // the line now, and 10 s of walking
            let rank = wormSpotVisible(g, 0, from, to) ? 0 : 10;
            if (occ.n) {
                for (let s = 0; s <= 5; s++) {
                    const sx = from + (to - from) * s / 5;
                    if (isHidden(sx, g.ridgeAt(0, sx) - 5)) rank += 1;
                }
            }
            if ((c.dir > 0 ? w - x : x) < 150) rank += 3;              // about to walk off the screen
            if (rank < bestRank) { bestRank = rank; best = x; }
            if (rank === 0) break;
        }
        return best;
    }

    function spawnCaravan(g, opts) {
        const o = opts || {};
        const c = caravan;
        c.dir = o.dir < 0 ? -1 : (o.dir > 0 ? 1 : (Math.random() < 0.5 ? -1 : 1));
        c.speed = rand(11, 15);
        c.n = 4 + Math.floor(Math.random() * 3);
        let off = 0;
        for (let k = 0; k < c.n; k++) {
            const m = c.camels[k];
            m.off = off;
            off += rand(14, 18);
            m.h = rand(9, 12);
            m.phase = rand(0, TWO_PI);
            m.omega = TWO_PI * c.speed / (0.9 * m.h);          // one stride per 0.9 camel heights of ground
            m.rider = (k & 1) === 0;
        }
        c.tail = c.camels[c.n - 1].off;
        c.lead = typeof o.x === 'number' && isFinite(o.x) ? o.x : pickCaravanLead(g, c);
        c.age = 0;
        c.leave = -1;
        c.on = true;
    }

    function updateCaravan(g, time) {
        if (g.layers < 1) return;
        const c = caravan;
        if (!c.on) {
            if (requests.caravan) {
                const request = requests.caravan;
                requests.caravan = null;
                spawnCaravan(g, request);
            } else if (time >= nextCaravanTime) {
                if (calmFor(g, CRITTER_CALM_MS)) {
                    spawnCaravan(g, null);
                    nextCaravanTime = nextCaravanAfter(time);
                } else {
                    nextCaravanTime = time + 2000;
                }
            }
            if (!c.on) return;
        }
        c.age += sdt;
        c.lead += c.dir * c.speed * sdt;
        for (let k = 0; k < c.n; k++) c.camels[k].phase += c.camels[k].omega * sdt;
        if (c.leave < 0 && g.stormActive) c.leave = 0;           // the storm starts: the line dissolves
        if (c.leave >= 0) {
            c.leave += sdt;
            if (c.leave >= CARAVAN_FADE) { c.on = false; return; }
        }
        const last = c.lead - c.dir * c.tail;                     // the trailing camel decides when it is gone
        if (c.dir > 0 ? last > g.width + 40 : last < -40) c.on = false;
    }

    // One camel in its own frame: origin on the ground under the belly, +x forward, -y up, height h.
    // Everything is drawn with the current fill and stroke colour, so the caller can paint the
    // silhouette twice, once as a rim of light and once dark.
    function traceCamel(ctx, m) {
        const h = m.h;
        const belly = -0.46 * h;                      // the legs are 0.46 h long
        const by = belly - 0.15 * h;                  // body centre
        ctx.lineWidth = 1;
        ctx.beginPath();
        for (let k = 0; k < 4; k++) {                 // two alternating pairs
            const ph = m.phase + (k === 0 || k === 3 ? 0 : Math.PI);
            const lx = CAMEL_LEG_X[k] * h;
            ctx.moveTo(lx, belly);
            ctx.lineTo(lx + Math.sin(ph) * 0.14 * h, -Math.max(0, Math.cos(ph)) * 0.1 * h);
        }
        ctx.moveTo(-0.48 * h, by - 0.05 * h);         // tail
        ctx.lineTo(-0.56 * h, by + 0.2 * h);
        ctx.stroke();
        ctx.lineWidth = Math.max(1.2, 0.17 * h);      // neck
        ctx.beginPath();
        ctx.moveTo(0.36 * h, by - 0.06 * h);
        ctx.quadraticCurveTo(0.6 * h, by - 0.16 * h, 0.7 * h, -0.93 * h);
        ctx.stroke();
        ctx.lineWidth = Math.max(1, 0.12 * h);        // head
        ctx.beginPath();
        ctx.moveTo(0.68 * h, -0.95 * h);
        ctx.quadraticCurveTo(0.8 * h, -0.99 * h, 0.91 * h, -0.9 * h);
        ctx.stroke();
        ctx.beginPath();
        ctx.moveTo(0.5 * h, by);
        ctx.ellipse(0, by, 0.5 * h, 0.19 * h, 0, 0, TWO_PI);             // body
        ctx.moveTo(-0.05 * h, by - 0.17 * h);
        ctx.ellipse(-0.18 * h, by - 0.17 * h, 0.13 * h, 0.17 * h, 0, 0, TWO_PI);   // hump
        ctx.moveTo(0.3 * h, by - 0.15 * h);
        ctx.ellipse(0.17 * h, by - 0.15 * h, 0.13 * h, 0.16 * h, 0, 0, TWO_PI);    // hump
        if (m.rider) {
            ctx.moveTo(0.075 * h, -1.07 * h);
            ctx.ellipse(-0.005 * h, -1.07 * h, 0.08 * h, 0.16 * h, 0, 0, TWO_PI);  // rider
            ctx.moveTo(0.075 * h, -1.29 * h);
            ctx.arc(0.01 * h, -1.29 * h, 0.065 * h, 0, TWO_PI);                    // head
        }
        ctx.fill();
    }

    function drawCaravan(ctx, g, fade) {
        const c = caravan;
        if (!c.on || g.layers < 1) return;
        const fadeIn = smoothstep(0, CARAVAN_FADE, c.age);
        const fadeOut = c.leave < 0 ? 1 : 1 - smoothstep(0, CARAVAN_FADE, c.leave);
        const a = fade * fadeIn * fadeOut;
        if (a < 0.01) return;
        ctx.save();
        try {
            ctx.lineCap = 'round';
            ctx.lineJoin = 'round';
            for (let k = c.n - 1; k >= 0; k--) {
                const m = c.camels[k];
                const x = c.lead - c.dir * m.off;
                if (x < -30 || x > g.width + 30) continue;
                const y = g.ridgeAt(0, x) + Math.sin(m.phase) * 0.6;           // bobbing
                const tilt = Math.atan2(g.ridgeAt(0, x + 4) - g.ridgeAt(0, x - 4), 8);
                for (let pass = 0; pass < 2; pass++) {
                    // pass 0: the rim of light on the sun side, one px off; pass 1: the dark silhouette
                    ctx.fillStyle = ctx.strokeStyle = pass === 0 ? 'rgb(255, 210, 150)' : 'rgb(26, 14, 6)';
                    ctx.globalAlpha = (pass === 0 ? 0.22 : 0.9) * a;
                    ctx.save();
                    try {
                        ctx.translate(x + (pass === 0 ? SUN_DIR_X : 0), y + (pass === 0 ? SUN_DIR_Y : 0));
                        ctx.rotate(tilt);
                        ctx.scale(c.dir, 1);
                        traceCamel(ctx, m);
                    } finally {
                        ctx.restore();
                    }
                }
            }
        } finally {
            ctx.restore();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  DEBUG
    // ═══════════════════════════════════════════════════════════════

    // Where the first visible creature of each family is ([x, y] in scene px), for a harness that
    // wants to look at it. The worm is [entry x, dive x, ridge] (0 far, 1 mid); occluders is the
    // number of chat rectangles the latest placement saw. vultures lists every bird in view and
    // thermal is the centre they circle (before the sway).
    function where() {
        const out = { vulture: null, vultures: [], thermal: [thermal.x, thermal.y], tumbleweed: null, beetle: null, caravan: null, worm: null, occluders: occ.n };
        if (worm) out.worm = [worm.x0, worm.x1, worm.e];
        for (let j = 0; j < birds.length; j++) {
            if (birds[j].alpha <= 0.01) continue;
            if (!out.vulture) out.vulture = [birds[j].x, birds[j].y];
            out.vultures.push([birds[j].x, birds[j].y]);
        }
        for (let i = 0; i < weeds.length && !out.tumbleweed; i++) {
            if (weeds[i].on) out.tumbleweed = [weeds[i].x, weeds[i].y];
        }
        if (beetle.on && geom) out.beetle = [beetle.x, geom.ridgeAt(1, beetle.x)];
        if (caravan.on && geom) out.caravan = [caravan.lead, geom.ridgeAt(0, caravan.lead)];
        return out;
    }
    debug.where = where;

    function strokeRidge(ctx, g, layer) {
        ctx.strokeStyle = 'rgba(0, 255, 180, 0.8)';
        ctx.lineWidth = 1;
        ctx.beginPath();
        for (let x = 0; x <= g.width; x += 4) {
            const y = g.ridgeAt(layer, x);
            if (x === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
        }
        ctx.stroke();
    }

    // ═══════════════════════════════════════════════════════════════
    //  PUBLIC API
    // ═══════════════════════════════════════════════════════════════

    // A bug here must not stop the engine's loop (an exception in update or draw would end
    // it): the layer switches itself off, says so loudly, and the next reset() tries again.
    function fail(err) {
        broken = true;
        if (typeof console !== 'undefined' && console.error) console.error('[SandstormLife] switched off after an error:', err);
    }

    function beginFrame(dt, time, g) {
        dtFrames = clamp(finiteOr(dt, 1), 0, 2.5);
        sdt = dtFrames / 60;
        lastTime = finiteOr(time, lastTime);
        if (!started) {
            started = true;
            if (startTime === null) startTime = lastTime;
            nextWormTime = firstWormTime(startTime);
            nextTumbleTime = firstTumbleTime(startTime);
            nextBeetleTime = firstBeetleTime(startTime);
            nextCaravanTime = firstCaravanTime(startTime);
            prevStormActive = g.stormActive;         // a storm that is already on is not a storm starting
            crestStamp = -Infinity;
            rollFlock();                             // the vultures glide in over the next 3 s
        }
        for (let i = 0; i < layerShift.length; i++) {
            const top = i < g.layers ? g.layerTop(i) : NaN;
            layerShift[i] = isFinite(top) && isFinite(layerTopPrev[i]) ? top - layerTopPrev[i] : 0;
            layerTopPrev[i] = top;
        }
    }

    function update(dt, time, g) {
        if (broken || !g) return;
        try {
            geom = g;
            beginFrame(dt, time, g);
            updateWorm(g, lastTime);
            updateGrains(g);
            updateWisps(g);
            updateShimmer(g);
            updateVultures(g);
            updateTumbleweeds(g, lastTime);
            updateBeetle(g, lastTime);
            updateCaravan(g, lastTime);
            prevStormActive = g.stormActive;
        } catch (err) {
            fail(err);
        }
    }

    function draw(ctx, g, stage) {
        if (broken || !started || !ctx || !g) return;
        ctx.save();
        try {
            geom = g;
            const fade = 1 - STORM_FADE * g.stormIntensity;
            if (stage === -1) {
                drawVultures(ctx, g, fade);
            } else if (stage === 0) {
                drawCaravan(ctx, g, fade);
                drawWisps(ctx, g, 0);
                drawLayerLife(ctx, g, 0, fade);
            } else if (stage === 1) {
                drawLayerLife(ctx, g, 1, fade);
                drawTumbleweeds(ctx, g, fade);
                drawBeetle(ctx, g, fade);
                drawWisps(ctx, g, 1);
            } else if (stage === 2) {
                drawShimmer(ctx, g);
            }
            if (debug.ridges && stage >= 0 && stage < g.layers) strokeRidge(ctx, g, stage);
        } catch (err) {
            fail(err);
        } finally {
            ctx.restore();
        }
    }

    // What lives on one dune: the worm (clipped to that ridge), then dust and flying sand.
    function drawLayerLife(ctx, g, layer, fade) {
        if (worm && worm.e === layer) drawWorm(ctx, g, worm, fade);
        drawPuffs(ctx, layer, fade);
        drawSpray(ctx, layer, fade);
    }

    // The engine rebuilt its dunes, which it does for every small change of the footer or the chat box,
    // so the creatures outlive it (a worm in mid-breach would cost 75-150 s of nothing): they read
    // g.ridgeAt every frame and just stand on the new ridges. What was measured on the old ridges is
    // void: sand in the air, dust, wisps and the crest list go. Only a narrower canvas can strand a
    // creature outside it. A worm whose entry or dive point fell outside is aimed again like a new one
    // (clamping both ends would fold the arch into a pillar; nothing in its update looks at x), and a
    // beetle is pulled in to where its walk keeps it (the walk does that only while it walks). A ball or
    // a camel line heading left that now starts beyond the right edge is put back where a new one
    // starts; one heading right is dropped by its next update once it is 40 px past that edge. A plain
    // clamp to the canvas would shove the balls and camels that are legitimately entering or leaving on
    // every resize, even one that kept the width.
    function resize(g) {
        if (g) geom = g;
        clearPool(spray);
        clearPool(puffs);
        clearPool(wisps);
        crestList = [];
        crestStamp = -Infinity;
        layerTopPrev[0] = layerTopPrev[1] = layerTopPrev[2] = NaN;
        band = null;
        if (geom && isFinite(geom.width)) {
            const width = geom.width, lo = 24, hi = width - 24;        // (the margins aimWorm keeps)
            if (worm) {
                if (worm.x0 < lo || worm.x0 > hi || worm.x1 < lo || worm.x1 > hi) {
                    aimWorm(geom, clamp(0.20 * width, 160, 420) * worm.S, clamp(worm.x0, lo, hi), worm.dir);
                    worm.x0 = aim.x0;
                    worm.x1 = aim.x1;
                    worm.dir = aim.dir;
                }
                worm.humpFrom = clamp(worm.humpFrom, 8, width - 8);
            }
            if (beetle.on) beetle.x = Math.min(beetle.x, 0.94 * width);   // the upper end of its walk
            for (let i = 0; i < weeds.length; i++) {
                if (weeds[i].on && weeds[i].dir < 0) weeds[i].x = Math.min(weeds[i].x, width + 30);   // where a new ball starts
            }
            if (caravan.on && caravan.dir < 0) caravan.lead = Math.min(caravan.lead, width);          // the leader at the edge
        }
        // The vultures fly in the sky and keep flying: their places come from the geometry of each
        // frame, so the next update puts them on the thermal of the new size. The chat moved too: look
        // at it again right away, starting from the default spot.
        thermal.due = 0;
        thermal.picked = false;
    }

    function reset(time) {
        clearPool(spray);
        clearPool(puffs);
        clearPool(wisps);
        worm = null;
        wormCount = 0;
        wormRequest = null;
        crestList = [];
        crestStamp = -Infinity;
        shimmerAmax = 0;
        windExcess = 0;
        layerTopPrev[0] = layerTopPrev[1] = layerTopPrev[2] = NaN;
        layerShift[0] = layerShift[1] = layerShift[2] = 0;
        requests.tumbleweed.length = 0;
        requests.beetle = requests.caravan = null;
        for (let i = 0; i < weeds.length; i++) weeds[i].on = false;
        beetle.on = false;
        caravan.on = false;
        clearFlock();
        thermal.due = 0;                             // the next glide-in looks at the chat and starts from the default spot
        thermal.picked = false;
        prevStormActive = false;
        started = false;
        broken = false;
        startTime = typeof time === 'number' && isFinite(time) ? time : null;
    }

    // Starts a worm now, whatever the weather. opts (all optional):
    // { layer: 0|1, dir: -1|1, x: starting x in CSS px }. Returns false while one is running.
    function forceWorm(opts) {
        if (broken || worm || wormRequest) return false;
        if (!started || !geom || geom.layers < 2 || geom.width < 120) {
            wormRequest = opts || {};                // begins with the next update
            return true;
        }
        try {
            beginWorm(geom, opts);
        } catch (err) {
            fail(err);
            return false;
        }
        return true;
    }

    // The three creatures below start with the next update, whatever the schedule or the
    // weather says (a beetle still burrows and a caravan still dissolves if a storm is on).
    // opts is optional; false means the family is at its maximum.
    //   forceTumbleweed({ dir: -1|1, x })   at most 2 balls
    //   forceBeetle({ dir: -1|1, x })       at most 1
    //   forceCaravan({ dir: -1|1, x })      at most 1; x is the leader's place
    function forceTumbleweed(opts) {
        if (broken || liveWeeds() + requests.tumbleweed.length >= MAX_TUMBLEWEEDS) return false;
        requests.tumbleweed.push(opts || {});
        return true;
    }

    function forceBeetle(opts) {
        if (broken || beetle.on || requests.beetle) return false;
        requests.beetle = opts || {};
        return true;
    }

    function forceCaravan(opts) {
        if (broken || caravan.on || requests.caravan) return false;
        requests.caravan = opts || {};
        return true;
    }

    function allFinite() {
        if (worm) {
            const w = worm;
            if (!isFinite(w.t + w.age + w.S + w.x0 + w.x1 + w.H + w.R + w.D + w.sHead + w.humpFrom + w.entry + w.dive)) return false;
            for (let i = 0; i < w.ripples.length; i++) {
                if (!isFinite(w.ripples[i].x + w.ripples[i].t)) return false;
            }
        }
        const pools = [spray, puffs, wisps];
        for (let k = 0; k < pools.length; k++) {
            const items = pools[k].items;
            for (let i = 0; i < items.length; i++) {
                const p = items[i];
                if (p.on && !isFinite(p.x + p.y + p.t + p.life)) return false;
                if (p.on && k !== 1 && !isFinite(p.vx + p.vy)) return false;
                if (p.on && k === 1 && !isFinite(p.r0 + p.r1)) return false;
            }
        }
        for (let i = 0; i < crestList.length; i++) {
            if (!isFinite(crestList[i].x + crestList[i].carry)) return false;
        }
        for (let j = 0; j < birds.length; j++) {
            const b = birds[j];
            if (!isFinite(b.alpha + b.x + b.y + b.ang + b.period + b.size + b.enterT + b.fvx + b.returnIn + b.flapT + b.flapIn)) return false;
        }
        if (!isFinite(thermal.x + thermal.y + thermal.tx + thermal.ty + thermal.due)) return false;
        for (let i = 0; i < weeds.length; i++) {
            const w = weeds[i];
            if (w.on && !isFinite(w.x + w.y + w.vx + w.vy + w.ang + w.r + w.hopIn)) return false;
        }
        if (beetle.on && !isFinite(beetle.x + beetle.t + beetle.left + beetle.life + beetle.sink + beetle.speed + beetle.gait + beetle.trickle)) return false;
        if (caravan.on) {
            const c = caravan;
            if (!isFinite(c.lead + c.tail + c.speed + c.age + c.leave)) return false;
            for (let k = 0; k < c.n; k++) {
                const m = c.camels[k];
                if (!isFinite(m.off + m.h + m.phase + m.omega)) return false;
            }
        }
        return isFinite(shimmerAmax + windExcess + nextTumbleTime + nextBeetleTime + nextCaravanTime);
    }

    function firstWeedX() {
        for (let i = 0; i < weeds.length; i++) if (weeds[i].on) return weeds[i].x;
        return null;
    }

    function stats() {
        return {
            attached: true,
            worm: worm ? worm.phase : null,
            wormCount: wormCount,
            spray: countLive(spray),
            puffs: countLive(puffs),
            wisps: countLive(wisps),
            shimmer: shimmerAmax,
            vultures: countVultures(),          // in view (alpha > 0)
            tumbleweeds: liveWeeds(),
            beetles: beetle.on ? 1 : 0,
            caravan: caravan.on,
            tumbleweedX: firstWeedX(),          // where the first ball is, null without one
            finite: allFinite()
        };
    }

    window.AuraGoSandstormLife = { update, draw, resize, reset, forceWorm, forceTumbleweed, forceBeetle, forceCaravan, stats, debug };
})();
