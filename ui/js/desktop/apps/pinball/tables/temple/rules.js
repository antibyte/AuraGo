// Lost Temple of the Sun: rule pack (plan 6.3). S-U-N opens the idol's doors and lights a mode, 20 pop hits light the
// orbit lock, three locks start GOLDEN MULTIBALL, four modes lead to the TEMPLE OF THE SUN wizard at the idol.
// Jewels (own design): lighting both jewels pays 100k and alternates between a bonus multiplier step and the kickback.
// Pure and deterministic: all randomness comes from ctx.rng, all output goes through ctx.
const SUN = ['sunS', 'sunU', 'sunN', 'sunStar'];
const JEWELS = ['jewelL', 'jewelR'];
const POPS_FOR_LOCK = 20;
const SHOT_BONUS = { leftRamp: 'ramps', rightRamp: 'ramps', leftOrbit: 'orbits', rightOrbit: 'orbits' };
const SHOT_FLASH = { idol: 'flIdol', leftRamp: 'flLeftRamp', rightRamp: 'flRightRamp' };
const EYES = { idle: '#ffcb65', doors: '#fff2b0', mode: '#8ad6a3', mb: '#ffcb65', wizard: '#ffffff' };

const big = (ctx, key, ms = 1500) => ctx.dmd('big', { text: key, vars: {} }, ms, 7);
const bank = (ctx, type) => ctx.cmd({ type, id: 'sun' });
const doors = (ctx, t, open) => { t.doors = open ? 1 : 0; ctx.cmd({ type: 'door', id: 'idolDoors', open }); };

function checkModeExtraBall(ctx, t) {
    if (ctx.player.modesCompleted.length < 2) { t.ebModes = false; return; }
    if (!t.ebModes) { t.ebModes = true; ctx.lightExtraBall(); }
}

// Doors shut, bank back up: used at ball start, after a mode was chosen and when a multiball ends.
function closeUp(ctx, t) {
    t.sun = [0, 0, 0, 0];
    doors(ctx, t, false);
    bank(ctx, 'resetBank');
}

function onSunDown(ctx, t) {
    t.sun = [1, 1, 1, 1];
    ctx.flash('flSun', 500);
    if (ctx.mbActive()) return;
    const wizard = ctx.player.wizardReady;
    ctx.lightModeStart();
    doors(ctx, t, true);
    big(ctx, wizard ? 'dmd_temple_sun' : 'dmd_temple_doors', 1800);
    ctx.show('sweepUp', { ms: 1200 });
    ctx.cue('combo');
    ctx.gi('upper', 'flicker', 500);
}

function onPop(ctx, t) {
    ctx.flash('flJungle', 140);
    const p = ctx.player;
    if (++t.pops < POPS_FOR_LOCK) return;
    t.pops = 0;
    if (p.lockLit || p.locks >= 3 || ctx.mbActive()) return;
    ctx.lightLock();
    ctx.cmd({ type: 'holeEnable', id: 'orbitLock', on: true });
    big(ctx, 'dmd_lock_lit');
    ctx.flash('flIdol', 300);
    ctx.say('lock_lit');
}

// Jewels: both lit pays 100k; odd completions advance the bonus multiplier, even ones relight the kickback.
function onJewel(ctx, t, id) {
    const i = JEWELS.indexOf(id);
    if (i < 0) return;
    t.jewels[i] = 1;
    if (!t.jewels.every(Boolean)) return;
    t.jewels = [0, 0];
    t.jewelDone++;
    ctx.score(100000);
    if (t.jewelDone % 2) ctx.advanceBonusX(); else ctx.lightKickback();
    ctx.show('sparkle', { ms: 1000 });
    ctx.cue('combo');
}

function onEvent(ctx, ev) {
    const t = ctx.player.table, g = ctx.game;
    if (t.mb && !ctx.mbActive()) { t.mb = false; closeUp(ctx, t); }
    switch (ev.type) {
    case 'bumper':
        onPop(ctx, t);
        if (g.mode.id === 'eclipse') ctx.gi('lower', 'flicker', 160);
        break;
    case 'drop': {
        const i = SUN.indexOf(ev.id);
        if (i >= 0) { t.sun[i] = 1; ctx.flash('flSun', 160); }
        if (g.mode.id === 'eclipse') ctx.gi('upper', 'flicker', 160);
        break;
    }
    case 'bankReset':
        t.sun = [0, 0, 0, 0];
        break;
    case 'bankDown':
        if (ev.id === 'sun') onSunDown(ctx, t);
        break;
    case 'standup':
        onJewel(ctx, t, ev.id);
        break;
    case 'spinner':
        if (g.mode.id === 'expedition') ctx.score(4000);
        break;
    case 'hole':
        // Remember whether a mode was already going when the ball dropped into the scoop (see onShot idol).
        if (ev.id === 'idolScoop') t.pre = g.mode.id || g.select || g.mb.state !== 'off' ? 1 : 0;
        break;
    default:
    }
    checkModeExtraBall(ctx, t);
}

function onShot(ctx, id) {
    const t = ctx.player.table, g = ctx.game;
    const shot = (ctx.table.layout.shots || []).find(s => s.id === id);
    if (SHOT_BONUS[id] && shot && !shot.bonus) ctx.bonus(SHOT_BONUS[id], 1);
    if (SHOT_FLASH[id]) ctx.flash(SHOT_FLASH[id], 250);
    if (id === 'idol') {
        if (g.mb.state === 'wizard') {
            t.mb = true;
            if (!t.doors) doors(ctx, t, true); // jackpots and the super jackpot are shot at the idol
            ctx.cue('wizard');
        } else if (!t.pre && (g.mode.id || g.select)) {
            closeUp(ctx, t);
        }
    }
    checkModeExtraBall(ctx, t);
}

function lamps(ctx, set) {
    const p = ctx.player, t = p.table, g = ctx.game;
    SUN.forEach((id, i) => set(id, t.sun[i] ? 'on' : 'off'));
    JEWELS.forEach((id, i) => set(id, t.jewels[i] ? 'on' : 'off'));
    set('doorsOpen', t.doors ? 'on' : 'off');
    const busy = g.mode.id || g.mb.state !== 'off';
    const eyes = busy ? 'fast' : t.doors ? 'on' : 'dim';
    set('idolEyeL', eyes);
    set('idolEyeR', eyes);
    set('spinnerLit', g.mode.id === 'expedition' ? 'on' : 'off');
}

function toy(g) {
    const t = g.players[g.turn].table, s = g.mb.state;
    const state = s === 'wizard' ? 'wizard' : s === 'running' ? 'mb' : g.mode.id ? 'mode' : t.doors ? 'doors' : 'idle';
    const glow = { idle: 0.15, doors: 0.7, mode: 0.85, mb: 1, wizard: 1 }[state];
    const sunRays = { idle: 0.1, doors: 0.35, mode: 0.5, mb: 0.8, wizard: 1 }[state];
    const dropped = t.sun.reduce((a, b) => a + b, 0) / SUN.length;
    return {
        idolEyes: { glow, color: EYES[state] }, doors: t.doors ? 1 : 0,
        pyramidGlow: Math.max(dropped * 0.6, state === 'idle' ? 0 : 0.5 + (glow - 0.5) * 0.5), sunRays
    };
}

export const rules = {
    points: {
        'bumper:*': 3010, 'sling:*': 510, 'spinner:*': 1010, 'rollover:*': 2510, 'standup:*': 5010, 'drop:*': 7510,
        'bankDown:*': 50000, 'hole:*': 25000, 'pathMade:*': 0, default: 1010
    },
    shotPoints: { leftRamp: 50000, rightRamp: 50000, leftOrbit: 30000, rightOrbit: 30000, idol: 50000 },
    bonusFrom: { 'standup:*': 'targets', 'drop:*': 'targets', 'bumper:*': 'bumpers', 'spinner:*': 'spins' },
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
    skill: { value: 250000, super: { on: 'pathMade:leftRamp', window: 8, value: 1000000 } },
    ballSave: { seconds: 8, lamp: 'shootAgain' },
    kickback: { device: 'kickbackL', lamp: 'kickbackL', litAtStart: true },
    extraBall: { shot: 'leftRamp', lamp: 'extraBall' },
    lock: { hole: 'orbitLock', shot: 'lock', physical: true, lamp: 'lockLit' },
    multiball: {
        name: 'mode_temple_mb', balls: 3, ballSave: 10, jackpotShots: ['leftRamp', 'rightRamp', 'leftOrbit', 'idol'],
        jackpotBase: 1000000, jackpotStep: 250000, superAfter: 4, superShot: 'idol', superWindow: 20, addABallShot: 'rightOrbit'
    },
    modeStart: { shot: 'idol', hole: 'idolScoop', lamp: 'modeStart' },
    modes: [
        { id: 'expedition', name: 'mode_temple_expedition', hint: 'dmd_hint_orbits', kind: 'count', targets: ['leftOrbit', 'rightOrbit'], count: 5, duration: 40, shotValue: 500000, step: 0, completeValue: 3000000 },
        { id: 'serpent', name: 'mode_temple_serpent', hint: 'dmd_hint_ramps', kind: 'count', targets: ['leftRamp', 'rightRamp'], count: 5, duration: 40, shotValue: 500000, step: 0, completeValue: 3000000 },
        { id: 'riddle', name: 'mode_temple_riddle', hint: 'dmd_hint_sequence', kind: 'sequence', targets: ['leftRamp', 'rightRamp', 'leftOrbit', 'rightOrbit'], count: 4, duration: 45, shotValue: 750000, step: 250000, completeValue: 4000000 },
        { id: 'eclipse', name: 'mode_temple_eclipse', hint: 'dmd_hint_bumpers', kind: 'count', targets: ['bumper:*', 'drop:*'], count: 25, duration: 35, shotValue: 100000, step: 0, completeValue: 2500000, gi: 'dim' }
    ],
    wizard: { id: 'wizard', name: 'mode_temple_wizard', balls: 4, ballSave: 20, jackpotValue: 5000000, step: 1000000, superShot: 'idol', superAfter: 6 },
    combos: { window: 3.0, base: 100000 },
    replay: 40000000,
    hooks: {
        init: () => ({ sun: [0, 0, 0, 0], jewels: [0, 0], jewelDone: 0, pops: 0, doors: 0, pre: 0, ebModes: false, mb: false }),
        onBallStart(ctx) {
            const t = ctx.player.table;
            t.mb = false; t.pre = 0;
            closeUp(ctx, t);
            ctx.cmd({ type: 'holeEnable', id: 'orbitLock', on: !!ctx.player.lockLit });
        },
        onLock(ctx, n) {
            ctx.flash('flIdol', 400);
            ctx.cmd({ type: 'holeEnable', id: 'orbitLock', on: false });
            if (n < 3) ctx.player.table.pops = 0;
        },
        onMultiball(ctx) {
            const t = ctx.player.table;
            t.mb = true;
            doors(ctx, t, true); // forced open: the idol is the super jackpot shot
            ctx.flash('flIdol', 500);
            ctx.say('multiball');
        },
        onEvent, onShot, lamps, toy
    }
};

export default rules;
