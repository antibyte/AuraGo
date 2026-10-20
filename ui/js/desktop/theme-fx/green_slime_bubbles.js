(function () {
    'use strict';

    // Bubble lifecycle of the Green Slime gel, a part of the simulation (see green_slime_sim.js):
    // nucleate (gas pocket in the gel) -> grow (dome swells, film thins) -> hold (quiver, faint
    // interference) -> burst (rim retracts, micro-droplets fly) -> crater (damped ring, rebound, heals).
    // All times are randomised; agitation spawns small fast bubbles. DOM-free.

    const kit = window.AuraGreenSlimeGeometry;
    const { clamp, lerp, smoothstep, wrap, cyclicDelta, easeOut, mulberry32, geometry } = kit;
    const { MAX_BUBBLES, MAX_BUBBLES_PER_WINDOW, SAG_BOTTOM, BUBBLE_STAGES, TWO_PI } = kit.constants;
    const { pointAt, project } = geometry;

    function create(core) {
        const { motion, rng, jitter, surfaces, state, counters, stageEntries } = core;
        const scratch = {};
        const scratchB = {};
        const centerProbe = {};
        let sequence = 0;

        function active() {
            let count = 0;
            for (const surface of surfaces.values()) count += surface.bubbles.length;
            return count;
        }

        function inCornerZone(lay, s) {
            for (const g of lay.segs) {
                if (!g.arc) continue;
                if (wrap(s - (g.start - 30), lay.length) <= g.len + 60) return true;
            }
            return false;
        }

        function nearDrip(surface, s) {
            const L = surface.layout.length;
            for (const drip of surface.drips) {
                if (Math.abs(cyclicDelta(s, surface.profile.nodeS[drip.node], L)) < 30) return true;
            }
            return false;
        }

        function overlaps(surface, s, base) {
            const L = surface.layout.length;
            for (const other of surface.bubbles) {
                if (Math.abs(cyclicDelta(s, other.s, L)) < base + other.base + 8) return true;
            }
            return false;
        }

        // Where a gas pocket may form: thick regions (bottom, lower sides) are likelier, never within
        // 30 px of a drip root or a corner, never hidden behind a higher window or overlay.
        function pickSite(surface, random, requested) {
            const lay = surface.layout, L = lay.length;
            for (let attempt = 0; attempt < 14; attempt++) {
                const s = random() * L;
                const R = core.radiusAt(surface, s);
                if (random() > Math.pow(R / SAG_BOTTOM, 2.2)) continue;
                if (inCornerZone(lay, s) || nearDrip(surface, s)) continue;
                const base = Math.min(clamp(requested, 3, 12), R * .45);
                if (base < 3 || overlaps(surface, s, base)) continue;
                const depth = R * .55;
                const p = pointAt(lay, s, scratch);
                if (core.blockedAt(surface, p.x + p.nx * depth, p.y + p.ny * depth)) continue;
                return { s, depth, base };
            }
            return null;
        }

        function createBubble(site, fast) {
            const durations = fast
                ? { nucleate: jitter(.09, .15), grow: jitter(.4, .6), hold: jitter(.06, .14), burst: jitter(.11, .16), crater: jitter(.5, .8) }
                : { nucleate: jitter(.5, .9), grow: jitter(1.4, 3.2), hold: jitter(.2, .8), burst: jitter(.11, .16), crater: jitter(.675, 1.125) };
            return { id: ++sequence, s: site.s, depth: site.depth, base: site.base, fast, stage: 'nucleate', time: 0, dur: durations,
                phase: rng() * TWO_PI, rupture: rng() * 2 - 1 };
        }

        // Spawns a bubble now: { windowId?, at?: {x, y}, base?, fast? }. Returns its id or null.
        function spawn(opts) {
            opts = opts || {};
            const env = state.env;
            if (!motion || !env.windows.length || active() >= MAX_BUBBLES) return null;
            let surface = null;
            if (opts.windowId != null) surface = surfaces.get(String(opts.windowId));
            else if (opts.at) {
                for (let i = env.windows.length - 1; i >= 0 && !surface; i--) {
                    const candidate = surfaces.get(String(env.windows[i].id));
                    if (!candidate || !candidate.layout) continue;
                    const hit = project(candidate.layout, opts.at.x, opts.at.y, scratch);
                    if (hit.dist > 0 && hit.dist < core.radiusAt(candidate, hit.s)) surface = candidate;
                }
            } else {
                const open = [...surfaces.values()].filter(item => item.layout && item.grow > .5 && item.bubbles.length < MAX_BUBBLES_PER_WINDOW);
                surface = open.length ? open[Math.floor(rng() * open.length)] : null;
            }
            if (!surface || !surface.layout || surface.bubbles.length >= MAX_BUBBLES_PER_WINDOW) return null;
            const fast = !!opts.fast;
            const requested = opts.base != null ? Number(opts.base) : fast ? jitter(3, 5) : jitter(4, 12);
            let site = null;
            if (opts.at && Number.isFinite(opts.at.x) && Number.isFinite(opts.at.y)) {
                const hit = project(surface.layout, opts.at.x, opts.at.y, scratch);
                const R = core.radiusAt(surface, hit.s);
                const base = Math.min(clamp(requested, 2.5, 12), R * .45);
                if (hit.dist > 0 && hit.dist < R && base >= 2.5 && !overlaps(surface, hit.s, base)) {
                    site = { s: hit.s, depth: clamp(hit.dist, base * .9, Math.max(base * .9, R - base)), base };
                }
            } else site = pickSite(surface, rng, requested);
            if (!site) return null;
            const bubble = createBubble(site, fast);
            surface.bubbles.push(bubble);
            counters.bubbleCount++;
            stageEntries.nucleate++;
            return bubble.id;
        }

        function center(surface, bubble, out) {
            const p = pointAt(surface.layout, bubble.s, scratchB);
            const depth = Math.min(bubble.depth, Math.max(2, core.radiusAt(surface, bubble.s) - bubble.base * .9));
            out.x = p.x + p.nx * depth;
            out.y = p.y + p.ny * depth;
            return out;
        }

        function enter(surface, bubble, stage) {
            bubble.stage = stage;
            bubble.time = 0;
            stageEntries[stage]++;
            if (stage === 'burst') {
                counters.bubbleBursts++;
                // Micro-droplets are flung from the retracting film, upward-biased.
                center(surface, bubble, scratch);
                const count = bubble.base < 5 ? 3 + Math.floor(rng() * 2) : 3 + Math.floor(rng() * 5);
                for (let i = 0; i < count; i++) {
                    const angle = rng() * TWO_PI;
                    const speed = jitter(80, 220);
                    core.addSpray(scratch.x + Math.cos(angle) * bubble.base * .8, scratch.y + Math.sin(angle) * bubble.base * .8 - bubble.base * .3,
                        jitter(.8, 2.2) * clamp(bubble.base / 9, .6, 1), Math.cos(angle) * speed, Math.sin(angle) * speed * .7 - jitter(60, 150), jitter(.5, .85));
                }
            } else if (stage === 'crater') {
                core.rippleImpulse(surface, bubble.s, 1.2 + bubble.base * .14, 14 + bubble.base * 1.2);
                // The small central rebound pulls the frame back for a moment.
                surface.pending.push({ at: state.clock + .12, kind: 'ripple', s: bubble.s, amplitude: -(.6 + bubble.base * .05), width: 12 });
            }
        }

        function step(surface, dt) {
            for (let i = surface.bubbles.length - 1; i >= 0; i--) {
                const bubble = surface.bubbles[i];
                bubble.time += dt;
                let guard = 5;
                while (bubble.time >= bubble.dur[bubble.stage] && guard-- > 0) {
                    const rest = bubble.time - bubble.dur[bubble.stage];
                    const next = BUBBLE_STAGES[BUBBLE_STAGES.indexOf(bubble.stage) + 1];
                    if (!next) { surface.bubbles.splice(i, 1); break; }
                    enter(surface, bubble, next);
                    bubble.time = rest;
                }
            }
        }

        // What the renderer needs of one bubble (see the scene contract in the design notes).
        function view(surface, bubble) {
            const stage = bubble.stage;
            const u = clamp(bubble.time / bubble.dur[stage], 0, 1);
            const seconds = bubble.time;
            const out = { id: bubble.id, x: 0, y: 0, base: bubble.base, r: bubble.base, dome: 0, stage, t: u,
                hole: 0, holeR: 0, hx: 0, hy: 0, crater: 0, rebound: 0, rim: 0, film: 1, interference: 0 };
            const c = center(surface, bubble, centerProbe);
            let lift = 0;
            if (stage === 'nucleate') {
                out.r = lerp(1, bubble.base * .4, easeOut(u));
                lift = 2.4 * u;
            } else if (stage === 'grow') {
                const g = easeOut(u);
                const wobble = .045 * (1 - u) * Math.sin(TWO_PI * 9 * seconds + bubble.phase);
                out.r = bubble.base * (.4 + .6 * g) * (1 + wobble);
                out.dome = clamp(g * (1 + wobble * 2), 0, 1.02);
                out.film = 1 - .8 * g;
                lift = 2.4 * (1 - g);
            } else if (stage === 'hold') {
                const quiver = Math.sin(TWO_PI * 17 * seconds + bubble.phase);
                out.r = bubble.base * (1 + .014 * quiver);
                out.dome = 1 + .012 * quiver;
                out.film = .2 - .12 * u;
                out.interference = smoothstep(.15, .9, u);
            } else if (stage === 'burst') {
                out.r = bubble.base;
                out.dome = 1 - .35 * u;
                out.film = .08;
                out.hole = Math.pow(u, .85);
                out.holeR = out.hole * 1.5 * bubble.base;
                out.rim = lerp(1.2, 3.5, u);
                out.hx = c.x + bubble.base * .22 * bubble.rupture;
                out.hy = c.y - bubble.base * .3;
            } else {
                out.r = bubble.base;
                out.crater = Math.exp(-3.6 * seconds) * Math.cos(TWO_PI * 3.2 * seconds);
                out.rebound = .6 * Math.exp(-Math.pow((seconds - .12) / .075, 2))
                    + (seconds > .12 ? .18 * Math.exp(-4 * seconds) * Math.sin(TWO_PI * 2.4 * (seconds - .12)) : 0);
            }
            out.x = c.x;
            out.y = c.y - lift;
            return out;
        }

        // Static mode keeps one deterministic, half-grown bubble per window.
        function frozen(surface) {
            const key = surface.profile.width + 'x' + surface.profile.height + ':' + surface.profile.radii.join(',');
            if (surface.frozenKey !== key) {
                surface.frozenKey = key;
                const site = pickSite(surface, mulberry32(surface.hash ^ 0x9e3779b9), 7);
                surface.frozen = site ? { id: -1, s: site.s, depth: site.depth, base: site.base, fast: false, stage: 'grow', time: 1.3,
                    dur: { nucleate: .7, grow: 2.4, hold: .5, burst: .13, crater: .9 }, phase: 1.3, rupture: 0 } : null;
            }
            return surface.frozen ? view(surface, surface.frozen) : null;
        }

        // Poisson process, mean interval ~9 s per window.
        function spawnAmbient(dt) {
            const now = state.env.now;
            for (const surface of surfaces.values()) {
                if (surface.age < 1.5 || surface.grow < .95 || now - surface.dragAt < .5) continue;
                if (rng() < 1 - Math.exp(-dt / 9)) spawn({ windowId: surface.id });
            }
        }

        // Resizing stretches the perimeter: bubbles move along proportionally instead of jumping.
        function rescale(surface, ratio) {
            for (const bubble of surface.bubbles) bubble.s *= ratio;
        }

        function report() {
            const stages = { nucleate: 0, grow: 0, hold: 0, burst: 0, crater: 0 };
            const samples = [];
            let count = 0;
            for (const surface of surfaces.values()) {
                for (const bubble of surface.bubbles) {
                    stages[bubble.stage]++;
                    count++;
                    // Where a test can look at it (screen coordinates of the dome centre).
                    const shown = view(surface, bubble);
                    samples.push({ id: bubble.id, window: surface.id, stage: bubble.stage, t: shown.t, x: shown.x, y: shown.y, r: shown.r, base: bubble.base });
                }
            }
            return { stages, active: count, samples };
        }

        return { active, spawn, step, view, frozen, spawnAmbient, rescale, report };
    }

    window.AuraGreenSlimeBubbles = Object.freeze({ create });
})();
