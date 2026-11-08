// AuraPinball DMD event scenes (awards, multiball, lock, tilt, ...). Pure functions of (draw API, params, t). DOM-free.
// Params use i18n keys (e.g. 'dmd_jackpot') or literal strings; scores are plain numbers.

export const ease = p => { const c = Math.max(0, Math.min(1, p)); return c * c * (3 - 2 * c); };
const easeOut = p => { const c = Math.max(0, Math.min(1, p)); return 1 - (1 - c) * (1 - c); };
const TAU = Math.PI * 2;
const sparkSeed = (t, salt = 7) => Math.floor(t * 12) * 131 + salt;

// Count-up value for a score: reaches `value` after `dur` seconds, in multiples of 10.
export function countUp(value, t, dur = 0.9, delay = 0) {
    const v = Math.max(0, Math.floor(value || 0)), p = (t - delay) / dur;
    if (p >= 1) return v;
    if (p <= 0) return 0;
    return Math.min(v, Math.round(v * easeOut(p) / 10) * 10);
}

export function starPoints(spikes = 5, outer = 1, inner = 0.45, rot = -Math.PI / 2) {
    const pts = [];
    for (let i = 0; i < spikes * 2; i++) {
        const r = i % 2 ? inner : outer, a = rot + i * Math.PI / spikes;
        pts.push([Math.cos(a) * r, Math.sin(a) * r]);
    }
    pts.push(pts[0]);
    return pts;
}
export const STAR = starPoints(5, 1, 0.45);
export const SPARK = starPoints(4, 1, 0.2);
export const LOCK_BODY = [[-7, -2], [7, -2], [7, 9], [-7, 9], [-7, -2]];
export const LOCK_SHACKLE = [[-4, -2], [-4, -7], [-2, -10], [2, -10], [4, -7], [4, -2]];
export const ARROW_UP = [[0, -6], [6, 2], [2, 2], [2, 7], [-2, 7], [-2, 2], [-6, 2], [0, -6]];

// A word that zooms in through the four sizes, then holds. Returns the drawn box.
export function zoomWord(d, text, t, cy, maxSize = 'xl', o = {}) {
    const order = ['s', 'm', 'l', 'xl'], last = Math.max(0, order.indexOf(maxSize));
    const size = order[Math.min(last, Math.floor(t / 0.08))];
    return d.text(text, o.x ?? 96, cy, { size, align: 'center', valign: 'middle', maxWidth: o.maxWidth || 184, level: o.level ?? 3, vars: o.vars });
}

// Rays that fly outward from a centre.
export function burst(d, cx, cy, t, o = {}) {
    const rays = o.rays || 12, rMax = o.rMax || 26, spin = o.spin || 0;
    const reach = Math.min(rMax, t * (o.speed || 46));
    if (reach <= 0) return;
    const fade = t > (o.hold || 0.9) ? 1 : 3;
    for (let i = 0; i < rays; i++) {
        const a = spin * t + (i * TAU) / rays, inner = Math.max(0, reach - 8 - (i % 2) * 4);
        d.line(cx + Math.cos(a) * inner, cy + Math.sin(a) * inner * 0.8, cx + Math.cos(a) * reach, cy + Math.sin(a) * reach * 0.8, i % 2 ? 2 : fade);
    }
}

function border(d, t, rate = 8, level = 3) {
    d.rect(0, 0, 192, 48, (Math.floor(t * rate) % 2) ? level : 1, false);
    d.rect(2, 2, 188, 44, (Math.floor(t * rate) % 2) ? 1 : level, false);
}

function flash(d, t, until, rate = 14) {
    if (t < until && Math.floor(t * rate) % 2 === 0) d.invert();
}

function bigScene(text, o = {}) {
    return { ms: o.ms ?? 1800, priority: o.priority ?? 7, transition: o.transition || 'zoom', draw(d, p, t) { o.draw(d, p, t, text); } };
}

const jackpotDraw = (d, p, t, key, strong) => {
    d.sparkle(strong ? 26 : 12, sparkSeed(t), { x: 0, y: 0, w: 192, h: 48 });
    for (const cx of [22, 170]) {
        burst(d, cx, 24, t, { rays: strong ? 16 : 12, rMax: 24, spin: strong ? 1.6 : 0.6 });
        d.sprite(STAR, cx, 24, 7 + Math.sin(t * 9) * 2, t * (strong ? 3 : 1.5), 3, false);
    }
    zoomWord(d, p.text || key, t, 10, 'l', { maxWidth: strong ? 124 : 108 });
    d.number(countUp(p.value, t, 1.0, 0.25), 96, 26, { font: 'l', align: 'center' });
    flash(d, t, strong ? 0.5 : 0.36, 10);
    if (strong) border(d, t, 10);
};

export const EVENT_SCENES = {
    big: bigScene('', { draw(d, p, t) {
        const hasLower = p.sub || Number.isFinite(p.score);
        d.sparkle(18, sparkSeed(t, 3));
        zoomWord(d, p.text || '', t, hasLower ? 14 : 24, hasLower ? 'l' : 'xl', { vars: p.vars });
        if (p.sub) d.text(p.sub, 96, 33, { size: 'm', align: 'center', maxWidth: 184, vars: p.vars });
        else if (Number.isFinite(p.score)) d.number(countUp(p.score, t, 0.8, 0.3), 96, 33, { font: 'm', align: 'center' });
        flash(d, t, 0.3);
    } }),

    jackpot: { ms: 2400, priority: 7, transition: 'zoom', draw: (d, p, t) => jackpotDraw(d, p, t, 'dmd_jackpot', false) },
    superJackpot: { ms: 3000, priority: 7, transition: 'zoom', draw: (d, p, t) => jackpotDraw(d, p, t, 'dmd_super_jackpot', true) },

    lock: { ms: 1900, priority: 5, transition: 'wipeLeft', draw(d, p, t) {
        const closed = t > 0.45, lift = closed ? 0 : -4;
        d.sprite(LOCK_BODY, 26, 26, 1.6, 0, 3, true);
        d.sprite(LOCK_SHACKLE, 26, 26 + lift * 1.6, 1.6, 0, 3, false);
        if (closed && t < 0.9) burst(d, 26, 26, t - 0.45, { rays: 10, rMax: 20, speed: 60 });
        const key = p.lit ? 'dmd_lock_lit' : 'dmd_ball_locked';
        zoomWord(d, key, t, 18, 'l', { x: 112, maxWidth: 138, vars: { n: p.n ?? 1 } });
        for (let i = 0; i < 3; i++) d.circle(78 + i * 18, 40, 4, i < (p.n ?? 0) ? 3 : 1, i < (p.n ?? 0));
    } },

    multiball: { ms: 2600, priority: 7, transition: 'zoom', draw(d, p, t) {
        for (let i = 0; i < 6; i++) {
            const y = 54 - ((t * 64 + i * 19) % 62), x = 18 + i * 31 + Math.sin(t * 3 + i) * 3;
            d.circle(x, y, 5, i % 2 ? 2 : 3, i % 3 === 0);
        }
        const word = Math.floor(t * 8) % 2 === 0 || t < 0.5 ? 3 : 2;
        zoomWord(d, p.text || 'dmd_multiball', t, 24, 'xl', { level: word });
        flash(d, t, 0.3);
    } },

    modeStart: { ms: 2300, priority: 5, transition: 'wipeRight', draw(d, p, t) {
        const sweep = Math.round(ease(t / 0.5) * 192);
        d.rect(0, 0, sweep, 1, 3); d.rect(192 - sweep, 47, sweep, 1, 3);
        zoomWord(d, p.name || '', t, p.hint ? 14 : 24, 'l', { vars: p.vars });
        if (p.hint) d.text(p.hint, 96, 34, { size: 's', align: 'center', maxWidth: 184 });
        d.sparkle(8, sparkSeed(t, 11));
    } },

    modeComplete: { ms: 2400, priority: 5, transition: 'zoom', draw(d, p, t) {
        d.sparkle(20, sparkSeed(t, 5));
        zoomWord(d, 'dmd_mode_complete', t, 10, 'l');
        if (p.name) d.text(p.name, 96, 20, { size: 's', align: 'center', maxWidth: 184 });
        d.number(countUp(p.score, t, 1.0, 0.3), 96, 31, { font: 'm', align: 'center' });
        flash(d, t, 0.3);
    } },

    modeTotal: { ms: 2200, priority: 5, transition: 'wipeUp', draw(d, p, t) {
        d.text('dmd_mode_total', 96, 3, { size: 'm', align: 'center' });
        d.number(countUp(p.score, t, 1.1, 0.2), 96, 22, { font: 'l', align: 'center' });
        if (t > 1.3) d.sparkle(10, sparkSeed(t, 2));
    } },

    skillShot: { ms: 2000, priority: 5, transition: 'zoom', draw(d, p, t) {
        const r = 4 + ((t * 14) % 10);
        for (const cx of [20, 172]) { d.circle(cx, 24, r, 2); d.circle(cx, 24, 3, 3, true); }
        zoomWord(d, p.super ? 'dmd_super_skill_shot' : 'dmd_skill_shot', t, 13, 'l', { maxWidth: 124 });
        d.number(countUp(p.value, t, 0.8, 0.25), 96, 31, { font: 'm', align: 'center' });
        flash(d, t, 0.25);
    } },

    ballSaved: { ms: 1700, priority: 5, transition: 'wipeLeft', draw(d, p, t) {
        const x = 12 + ((t * 120) % 168), y = 40 - Math.abs(Math.sin(t * 7)) * 22;
        d.circle(x, y, 4, 3, true);
        zoomWord(d, 'dmd_ball_saved', t, 18, 'l');
        d.rect(0, 46, Math.round(192 * Math.min(1, t / 1.4)), 2, 2);
    } },

    extraBall: { ms: 2300, priority: 5, transition: 'zoom', draw(d, p, t) {
        for (const cx of [20, 172]) d.sprite(STAR, cx, 24, 11 + Math.sin(t * 8) * 2, t * 2.4, 3, true);
        const key = p.lit ? 'dmd_extra_ball_lit' : 'dmd_extra_ball';
        zoomWord(d, key, t, 24, p.lit ? 'l' : 'xl', { maxWidth: 124, level: Math.floor(t * 8) % 2 ? 2 : 3 });
        d.sparkle(14, sparkSeed(t, 9));
    } },

    replay: { ms: 2000, priority: 5, transition: 'zoom', draw(d, p, t) {
        d.sparkle(30, sparkSeed(t, 13));
        zoomWord(d, 'dmd_replay', t, 24, 'xl', { level: Math.floor(t * 6) % 2 ? 2 : 3 });
        flash(d, t, 0.3);
    } },

    combo: { ms: 1400, priority: 3, transition: 'wipeLeft', draw(d, p, t) {
        zoomWord(d, 'dmd_combo', t, 12, 'l', { vars: { n: p.n ?? 2 } });
        if (Number.isFinite(p.value)) d.number(countUp(p.value, t, 0.5), 96, 30, { font: 'm', align: 'center' });
        d.rect(0, 45, Math.round(192 * (1 - t / 1.4)), 2, 3);
    } },

    tilt: { ms: 3500, priority: 9, transition: 'cut', draw(d, p, t) {
        border(d, t, 9);
        if (Math.floor(t * 5) % 3 !== 2) d.text('dmd_tilt', 96, 24, { size: 'xl', align: 'center', valign: 'middle' });
        if (Math.floor(t * 9) % 2) d.rect(8, 8, 176, 32, 3, false);
    } },

    tiltWarning: { ms: 1300, priority: 7, transition: 'cut', draw(d, p, t) {
        border(d, t, 12);
        if (Math.floor(t * 8) % 2 === 0) d.text('dmd_danger', 96, 24, { size: 'xl', align: 'center', valign: 'middle', maxWidth: 176 });
        if (p.n) for (let i = 0; i < 3; i++) d.circle(80 + i * 16, 42, 2, i < p.n ? 3 : 1, i < p.n);
    } },

    shootAgain: { ms: 1700, priority: 5, transition: 'wipeUp', draw(d, p, t) {
        for (const cx of [14, 178]) d.sprite(ARROW_UP, cx, 24 - ((t * 40) % 24) + 12, 1.4, 0, 3, true);
        zoomWord(d, 'dmd_shoot_again', t, 24, 'l', { maxWidth: 140 });
    } },

    wizardStart: { ms: 3400, priority: 7, transition: 'zoom', draw(d, p, t) {
        d.sparkle(24, sparkSeed(t, 17));
        burst(d, 96, 24, t, { rays: 24, rMax: 60, spin: 0.8, hold: 1.5, speed: 40 });
        const scan = Math.floor((t * 70) % 192);
        d.line(scan, 0, scan, 47, 2);
        zoomWord(d, p.name || 'dmd_wizard_lit', t, 24, 'l', { vars: p.vars });
        border(d, t, 6);
    } }
};
