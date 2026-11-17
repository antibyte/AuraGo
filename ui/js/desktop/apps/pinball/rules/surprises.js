// AuraPinball surprises & secret combos: deterministic hidden awards (Lazarus outlane save, Hyper Spinner frenzy,
// Bumper Mayhem, Warp Orbit loop and per-table 3-way Secret Combos). Pure, DOM-free, plain JSON state.
import { cur, pack, dmd, show, flash, cue, shake, saveActive, addScore } from './features.js';

const RECOVERY = new Set(['bumper', 'sling', 'standup', 'drop', 'spinner', 'hole', 'pathMade', 'captive', 'rollover']);

export function idleSurprise() {
    return {
        outlaneUntil: 0, lazarusDone: false,
        spinUntil: 0, spins: 0, spinOverdrive: 0, spinDone: false,
        popUntil: 0, pops: 0, popDone: false,
        loopUntil: 0, loops: 0, loopDone: false,
        seqStep: 0, seqUntil: 0, secretDone: false
    };
}

export function clearOutlaneSurprise(g) {
    if (g.surprise) g.surprise.outlaneUntil = 0;
}

function active(g) {
    return !!(g.surprise && pack(g).surprises && g.launched && g.time >= 0.5 && !cur(g).tilt);
}

function celebrateSurprise(g, text, points, showName, extra = {}) {
    const v = addScore(g, points, { bonus: 'combos' });
    dmd(g, 'surprise', { text, score: v }, extra.ms ?? 2200, 7);
    cue(g, 'surprise');
    show(g, showName, { ms: 1500, priority: 2, ...(extra.show || {}) });
    flash(g, 'all', extra.flashMs ?? 320);
    if (extra.shake) shake(g, extra.shake);
}

export function surpriseEvent(g, ev) {
    if (!ev || !active(g)) return;
    const s = g.surprise, pk = pack(g), p = cur(g);

    // 1. Lazarus / Close Call: unguarded outlane rollover survived back into play without ball-save.
    if (ev.type === 'rollover' && (ev.id === 'outlaneL' || ev.id === 'outlaneR')) {
        const kb = pk.kickback;
        const guarded = !!(kb && p.kickbackLit && ((/L$/i.test(kb.device) && ev.id === 'outlaneL') || (/R$/i.test(kb.device) && ev.id === 'outlaneR')));
        if (!guarded && !saveActive(g) && !s.lazarusDone) s.outlaneUntil = g.time + 3.5;
        return;
    }
    if (s.outlaneUntil > g.time && !s.lazarusDone && RECOVERY.has(ev.type)) {
        s.outlaneUntil = 0;
        s.lazarusDone = true;
        celebrateSurprise(g, 'dmd_surprise_lazarus', 350000, 'spiral', { shake: 0.003 });
    }

    // 2. Hyper Spinner Frenzy: 16+ revolutions within 2.4 s triggers 8 s overdrive.
    if (ev.type === 'spinner') {
        if (g.time < s.spinOverdrive) addScore(g, 5000);
        if (!s.spinDone) {
            if (g.time > s.spinUntil) { s.spins = 1; s.spinUntil = g.time + 2.4; }
            else s.spins++;
            if (s.spins >= 16) {
                s.spinDone = true;
                s.spinOverdrive = g.time + 8;
                celebrateSurprise(g, 'dmd_surprise_spinner', 300000, 'chase', { flashMs: 260 });
            }
        }
    }

    // 3. Bumper Mayhem: 14 rapid bumper hits within 4.0 s.
    if (ev.type === 'bumper' && !s.popDone) {
        if (g.time > s.popUntil) { s.pops = 1; s.popUntil = g.time + 4.0; }
        else s.pops++;
        if (s.pops >= 14) {
            s.popDone = true;
            celebrateSurprise(g, 'dmd_surprise_mayhem', 300000, 'ring', {
                shake: 0.003,
                show: { x: Number.isFinite(ev.x) ? ev.x : 0, y: Number.isFinite(ev.y) ? ev.y : 0.82 }
            });
        }
    }
}

export function surpriseShots(g, shots) {
    if (!shots || !shots.length || !active(g)) return;
    const s = g.surprise, cfg = pack(g).surprises;

    // 4. Warp Orbit Loop: 3 orbits chained within 9 s.
    if (!s.loopDone && shots.some(sh => sh.id === 'leftOrbit' || sh.id === 'rightOrbit')) {
        if (g.time > s.loopUntil) s.loops = 1;
        else s.loops++;
        s.loopUntil = g.time + 9.0;
        if (s.loops >= 3) {
            s.loopDone = true;
            celebrateSurprise(g, 'dmd_surprise_loop', 500000, 'wipeDiagonal', { ms: 2300, flashMs: 360 });
        }
    }

    // 5. Table-specific Secret Combo Easter Egg.
    const seq = cfg && typeof cfg === 'object' && Array.isArray(cfg.secret) ? cfg.secret : null;
    if (seq && seq.length >= 2 && !s.secretDone) {
        if (s.seqStep > 0 && g.time > s.seqUntil) s.seqStep = 0;
        if (shots.some(sh => sh.id === seq[s.seqStep])) {
            s.seqStep++;
            s.seqUntil = g.time + 12.0;
            if (s.seqStep >= seq.length) {
                s.secretDone = true;
                s.seqStep = 0;
                const p = cur(g), max = pack(g).bonusX?.max ?? 5;
                if (p.bonusX < max) p.bonusX++;
                celebrateSurprise(g, cfg.key || 'dmd_surprise_mystery', cfg.value ?? 750000, 'rainbow', { ms: 2600, flashMs: 450, shake: 0.004 });
            }
        } else if (s.seqStep > 0 && shots.some(sh => sh.id === seq[0])) {
            s.seqStep = 1;
            s.seqUntil = g.time + 12.0;
        }
    }
}
