// Clockwork Observatory: rule pack (plan 5.6 as tuned in S5). Every shot moves the clock forward, midnight starts MIDNIGHT MULTIBALL,
// S-T-A-R lights the gravity wells and latches the pendulum, the telescope starts modes, the orrery shot reverses the disc.
// Pure and deterministic: randomness comes from ctx.rng, timers from hooks.onTick (ctx.now), all output goes through ctx.
const STAR = ['starS', 'starT', 'starA', 'starR'];
const WELLS = ['wellMars', 'wellSaturn'];
const SHOTS = ['leftOrbit', 'comet', 'orrery', 'telescope', 'meridian', 'rightOrbit'];
const SHOT_BONUS = { comet: 'ramps', meridian: 'ramps', leftOrbit: 'orbits', rightOrbit: 'orbits' };
const SHOT_FLASH = { telescope: 'flTelescope', orrery: 'flOrrery', comet: 'flComet', meridian: 'flMeridian' };
// Disc speed in rad/s by state (the sign is discDir); the plan values scaled for the strong disc friction of this table.
const DISC_SPEED = { wizard: 24, multiball: 15, meteor: 17, transit: 9, conjunction: 3, supernova: 13, idle: 6 };
// The gravity slingshot throws at the mouth of the telescope channel (the channel walls block the straight line to the VUK).
export const SLING = Object.freeze({ at: [0.045, 0.590], speed: 1.6 });
const HOLD = 0.8, CAPTURE_WINDOW = 2, WELL_WINDOW = 10, LATCH = 12, ASSIST_REARM = 0.8;
const TICK_GAP = 0.6, SLING_VALUE = 1000000;

const big = (ctx, key, ms = 1500, vars = {}) => ctx.dmd('big', { text: key, vars }, ms, 7);
const magnet = (ctx, id, mode, extra = {}) => ctx.cmd({ type: 'magnet', id, mode, ...extra });
const wellsOff = ctx => WELLS.forEach(id => magnet(ctx, id, 'off'));
const wellsGrab = ctx => WELLS.forEach(id => magnet(ctx, id, 'grab'));

function checkModeExtraBall(ctx, t) {
    if (ctx.player.modesCompleted.length < 2) { t.ebModes = false; return; }
    if (!t.ebModes) { t.ebModes = true; ctx.lightExtraBall(); }
}

// What the disc and the pendulum should be doing now.
function discState(g) {
    if (g.mb.state === 'wizard') return 'wizard';
    if (g.mb.state === 'running') return 'multiball';
    return ({ meteor: 'meteor', transit: 'transit', conjunction: 'conjunction', supernova: 'supernova' })[g.mode.id] || 'idle';
}
const discSpeed = (g, t) => DISC_SPEED[discState(g)] * t.discDir;
const pendulumMode = (g, t) => g.mb.state === 'wizard' || t.latchUntil > g.time ? 'park' : g.mode.id === 'transit' ? 'fast' : 'swing';

// Commands only on change; `force` re-sends (ball start).
function sync(ctx, t, force = false) {
    const g = ctx.game, speed = discSpeed(g, t), pend = pendulumMode(g, t);
    if (force || t.discCmd !== speed) { t.discCmd = speed; ctx.cmd({ type: 'orrery', id: 'orrery', speed }); }
    if (force || t.pendCmd !== pend) { t.pendCmd = pend; ctx.cmd({ type: 'pendulum', id: 'pendulum', mode: pend }); }
}

// ---------------------------------------------------------------- clock
function midnight(ctx, t) {
    t.midnightPending = false;
    if (!ctx.startMultiball()) return;
    t.hour = 0;
    big(ctx, 'dmd_observatory_midnight', 1800);
    ctx.show('ring', { ms: 1600 });
}

function advance(ctx, t, n) {
    if (ctx.mbActive()) return;
    t.hour += n;
    if (t.hour >= 12) {
        if (ctx.game.select) { t.hour = 12; t.midnightPending = true; } else midnight(ctx, t);
        return;
    }
    ctx.dmd('big', { text: 'dmd_observatory_hour', vars: { n: t.hour } }, 900, 3);
}

// ---------------------------------------------------------------- S-T-A-R and the gravity wells
function onStar(ctx, t, id) {
    const i = STAR.indexOf(id);
    if (i < 0) return;
    t.star[i] = 1;
    if (!t.star.every(Boolean)) return;
    t.star = [0, 0, 0, 0];
    t.stars++;
    ctx.score(100000);
    if (!ctx.mbActive()) ctx.lightModeStart();
    advance(ctx, t, 1);
    t.wellsLit = true; t.wellWindow = ctx.now + WELL_WINDOW; t.hold = null;
    wellsGrab(ctx);
    t.latchUntil = ctx.now + LATCH;
    big(ctx, 'dmd_observatory_star');
    ctx.show('sparkle', { ms: 1000 });
    ctx.flash('flWells', 500);
    ctx.dmd('big', { text: 'dmd_observatory_wells', vars: {} }, 1200, 7);
    ctx.dmd('big', { text: 'dmd_observatory_latched', vars: {} }, 1200, 7);
    ctx.say('lock_lit');
}

function onMagnet(ctx, t, ev) {
    if (!WELLS.includes(ev.id)) return;
    if (ev.state === 'held') {
        t.hold = { id: ev.id, at: ctx.now + HOLD };
        ctx.flash('flWells', 300);
    } else if (ev.state === 'release') t.hold = null;
}

function slingAngle(ctx, id) {
    const w = (ctx.table.layout.devices || []).find(d => d.id === id);
    return w ? Math.atan2(SLING.at[1] - w.y, SLING.at[0] - w.x) : Math.PI / 2;
}

function onTick(ctx) {
    const t = ctx.player.table, g = ctx.game, now = ctx.now;
    if (t.hold && now >= t.hold.at) {
        magnet(ctx, t.hold.id, 'throw', { angle: slingAngle(ctx, t.hold.id), speed: SLING.speed });
        t.hold = null; t.capture = now + CAPTURE_WINDOW; t.rearm = now + ASSIST_REARM;
    }
    if (t.capture && now > t.capture) t.capture = 0;
    const assist = g.mode.id === 'conjunction' && !g.select;
    if (t.wellsLit && !t.hold && now >= t.wellWindow) { t.wellsLit = false; if (!assist) wellsOff(ctx); }
    if (assist && !t.assist) { t.assist = true; t.rearm = 0; wellsGrab(ctx); }
    else if (!assist && t.assist) { t.assist = false; if (!t.wellsLit && !t.hold) wellsOff(ctx); }
    else if (assist && t.rearm && now >= t.rearm && !t.hold) { t.rearm = 0; wellsGrab(ctx); }
    if (t.midnightPending && !g.select) midnight(ctx, t);
    sync(ctx, t);
}

function onEvent(ctx, ev) {
    const t = ctx.player.table, g = ctx.game;
    if (t.mb && !ctx.mbActive()) t.mb = false;
    switch (ev.type) {
    case 'standup': onStar(ctx, t, ev.id); break;
    case 'magnet': onMagnet(ctx, t, ev); break;
    case 'pendulum':
        ctx.flash('flPendulum', 200);
        if (ctx.now - t.lastTick >= TICK_GAP && !ctx.mbActive()) { t.lastTick = ctx.now; advance(ctx, t, 1); }
        break;
    case 'spinner':
        if (g.mode.id === 'meteor') ctx.score(4000);
        break;
    case 'hole':
        if (ev.id === 'telescope' && t.capture > ctx.now) {
            t.capture = 0;
            const v = ctx.score(SLING_VALUE);
            ctx.dmd('big', { text: 'dmd_observatory_slingshot', score: v, vars: {} }, 2000, 7);
            ctx.cue('jackpot'); ctx.show('radial', { ms: 1200 }); ctx.shake(0.002);
        }
        break;
    default:
    }
    checkModeExtraBall(ctx, t);
    sync(ctx, t);
}

function onShot(ctx, id) {
    const t = ctx.player.table;
    const shot = (ctx.table.layout.shots || []).find(s => s.id === id);
    if (SHOT_BONUS[id] && shot && !shot.bonus) ctx.bonus(SHOT_BONUS[id], 1);
    if (SHOT_FLASH[id]) ctx.flash(SHOT_FLASH[id], 250);
    if (id === 'orrery') { t.discDir *= -1; big(ctx, 'dmd_observatory_reverse', 1000); }
    advance(ctx, t, id === 'telescope' ? 2 : 1);
    checkModeExtraBall(ctx, t);
    sync(ctx, t);
}

function lamps(ctx, set) {
    const g = ctx.game, t = ctx.player.table, mb = g.mb.state === 'running';
    for (let k = 1; k <= 12; k++) set('clock' + k, mb ? 'fast' : k <= t.hour ? 'on' : k === t.hour + 1 ? 'slow' : 'off');
    STAR.forEach((id, i) => set(id, t.star[i] ? 'on' : 'off'));
    const well = id => t.hold && t.hold.id === id ? 'fast' : t.wellsLit ? 'slow' : 'off';
    set('wellMarsLit', well('wellMars')); set('wellSaturnLit', well('wellSaturn'));
    set('pendulumLit', ({ park: 'on', swing: 'slow', fast: 'fast' })[pendulumMode(g, t)]);
    set('spinnerLit', g.mode.id === 'meteor' ? 'on' : 'off');
}

// The toy state: C10 plus the dome shutter.
function toy(g) {
    const t = g.players[g.turn].table, s = g.mb.state, p = g.players[g.turn];
    const busy = s !== 'off' || !!g.mode.id;
    const speed = discSpeed(g, t), open = s !== 'off' ? 1 : 0;
    return {
        disc: { speed },
        orrery: { speed, glow: s === 'wizard' ? 1 : busy ? 0.8 : 0.3 },
        telescope: { glow: s !== 'off' ? 1 : (p.modeReady || g.select || t.wellsLit) ? 0.7 : 0.25, cap: open || p.modeReady || t.latchUntil > g.time ? 1 : 0 },
        gears: { hour: t.hour, run: s !== 'off' ? 1 : g.mode.id ? 0.6 : 0.25 },
        pendulum: { mode: pendulumMode(g, t) },
        clock: { hour: t.hour, midnight: s === 'running' || t.midnightPending },
        dome: { open }
    };
}

export const rules = {
    points: {
        'bumper:*': 3010, 'sling:*': 510, 'spinner:*': 1010, 'rollover:*': 2510, 'standup:*': 5010, 'pendulum:*': 7510,
        'orrery:*': 1010, 'hole:*': 25000, 'pathMade:*': 0, default: 1010
    },
    shotPoints: { comet: 50000, meridian: 50000, leftOrbit: 30000, rightOrbit: 30000, orrery: 40000, telescope: 50000 },
    bonusFrom: { 'standup:*': 'targets', 'pendulum:*': 'targets', 'bumper:*': 'bumpers', 'spinner:*': 'spins' },
    bonusCounters: [
        { id: 'ramps', key: 'dmd_bonus_ramps', value: 25000 },
        { id: 'orbits', key: 'dmd_bonus_orbits', value: 15000 },
        { id: 'targets', key: 'dmd_bonus_targets', value: 5000 },
        { id: 'bumpers', key: 'dmd_bonus_bumpers', value: 1000 },
        { id: 'spins', key: 'dmd_bonus_spins', value: 500 },
        { id: 'modes', key: 'dmd_bonus_modes', value: 250000 },
        { id: 'locks', key: 'dmd_bonus_locks', value: 50000 },
        { id: 'combos', key: 'dmd_bonus_combos', value: 50000 }
    ],
    topLanes: { switches: ['laneT1', 'laneT2', 'laneT3'], lamps: ['laneT1', 'laneT2', 'laneT3'], award: 'bonusX' },
    lowerLanes: { switches: ['inlaneL', 'outlaneL', 'inlaneR', 'outlaneR'], lamps: ['inlaneL', 'outlaneL', 'inlaneR', 'outlaneR'], award: 'kickback' },
    bonusX: { lamps: ['bonus2x', 'bonus3x', 'bonus4x', 'bonus5x'], max: 5 },
    skill: { value: 250000, super: { on: 'hole:telescope', window: 8, value: 1000000 } },
    ballSave: { seconds: 8, lamp: 'shootAgain' },
    kickback: { device: 'kickbackL', lamp: 'kickbackL', litAtStart: true },
    extraBall: { shot: 'telescope', lamp: 'extraBall' },
    multiball: {
        name: 'mode_observatory_mb', balls: 3, ballSave: 10, jackpotShots: ['comet', 'meridian', 'orrery', 'leftOrbit'],
        jackpotBase: 1000000, jackpotStep: 250000, superAfter: 4, superShot: 'telescope', superWindow: 20, addABallShot: 'rightOrbit'
    },
    modeStart: { shot: 'telescope', hole: 'telescope', lamp: 'modeStart' },
    modes: [
        { id: 'meteor', name: 'mode_observatory_meteor', hint: 'dmd_hint_ramps', kind: 'count', targets: ['comet', 'meridian'], count: 5, duration: 40, shotValue: 500000, step: 0, completeValue: 3000000 },
        { id: 'transit', name: 'mode_observatory_transit', hint: 'dmd_hint_sequence', kind: 'sequence', targets: ['leftOrbit', 'orrery', 'telescope', 'rightOrbit'], count: 4, duration: 45, shotValue: 750000, step: 250000, completeValue: 4000000 },
        { id: 'conjunction', name: 'mode_observatory_conjunction', hint: 'dmd_hint_lit', kind: 'roving', targets: SHOTS, count: 4, moveEvery: 5, duration: 45, shotValue: 750000, step: 0, completeValue: 4000000 },
        { id: 'supernova', name: 'mode_observatory_supernova', hint: 'dmd_hint_targets', kind: 'count', targets: ['bumper:*', 'standup:*', 'pendulum:*'], count: 20, duration: 35, shotValue: 100000, step: 0, completeValue: 2500000, gi: 'dim' }
    ],
    wizard: { id: 'wizard', name: 'mode_observatory_wizard', balls: 4, ballSave: 20, jackpotValue: 5000000, step: 1000000, superShot: 'telescope', superAfter: 6 },
    combos: { window: 3.0, base: 100000 },
    surprises: { secret: ['orrery', 'telescope', 'meridian'], key: 'dmd_secret_observatory', value: 750000 },
    replay: 40000000,
    hooks: {
        init: () => ({
            hour: 0, star: [0, 0, 0, 0], stars: 0, latchUntil: 0, wellsLit: false, wellWindow: 0, hold: null, capture: 0, assist: false, rearm: 0,
            midnightPending: false, discDir: 1, discCmd: null, pendCmd: null, lastTick: -1, mb: false, ebModes: false
        }),
        onBallStart(ctx) {
            const t = ctx.player.table;
            t.wellsLit = false; t.wellWindow = 0; t.hold = null; t.capture = 0; t.assist = false; t.rearm = 0; t.latchUntil = 0;
            t.midnightPending = false; t.mb = false; t.lastTick = -1;
            wellsOff(ctx);
            sync(ctx, t, true);
        },
        onMultiball(ctx) {
            ctx.player.table.mb = true;
            ctx.flash('flTelescope', 500);
            ctx.say('multiball');
            sync(ctx, ctx.player.table);
        },
        onEvent, onShot, onTick, lamps, toy
    }
};

export default rules;
