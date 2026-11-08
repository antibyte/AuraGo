// Cyber Neon Metropolis: rule pack (plan 6.1). FIREWALL lights the vault lock, three locks start GRIDLOCK MULTIBALL,
// N-E-O-N advances the bonus multiplier, four modes lead to the SYSTEM MELTDOWN wizard at the Data Core.
// Pure and deterministic: all randomness comes from ctx.rng, all output goes through ctx.
const LETTERS = ['neonN', 'neonE', 'neonO', 'neonN2'];
const FW = ['fw1', 'fw2', 'fw3'];
const UPLINK = ['uplinkU', 'uplinkP', 'uplinkL'];
const UPLINK_SCORE = 250000, UPLINK_REPEAT = 500000, UPLINK_MODE_SECONDS = 10;
const SPIN_RELIGHT = 10;
const SHOT_BONUS = { leftRamp: 'ramps', rightRamp: 'ramps', leftOrbit: 'orbits', rightOrbit: 'orbits' };
const SHOT_FLASH = { core: 'flCore', vault: 'flVault', leftRamp: 'flLeftRamp', rightRamp: 'flRightRamp' };
const COLORS = { idle: '#37d9ff', mode: '#ff48c4', mb: '#ffd24a', wizard: '#ffffff' };

const big = (ctx, key, ms = 1500) => ctx.dmd('big', { text: key, vars: {} }, ms, 7);
const bank = (ctx, type) => ctx.cmd({ type, id: 'firewall' });

function checkModeExtraBall(ctx, t) {
    const done = ctx.player.modesCompleted.length;
    if (done < 2) { t.ebModes = false; return; }
    if (!t.ebModes) { t.ebModes = true; ctx.lightExtraBall(); }
}

function onStandup(ctx, t, id) {
    const i = LETTERS.indexOf(id);
    if (i < 0) return;
    t.letters[i] = 1;
    if (!t.letters.every(Boolean)) return;
    t.letters = [0, 0, 0, 0];
    t.neon++;
    ctx.advanceBonusX();
    ctx.score(100000);
    ctx.lightModeStart();
    if (t.neon === 2) ctx.lightExtraBall();
    else if (t.neon === 3) ctx.lightKickback();
    big(ctx, 'dmd_neon_letters');
    ctx.show('sparkle', { ms: 1200 });
}

// U-P-L: the upper flipper's targets. Every completion pays 250k. Level 1 also lights the extra ball (500k more if it is
// already lit), later levels pay 500k more;
// a running mode gets 10 s more, Gridlock multiball re-lights every jackpot.
function onUplink(ctx, t, id) {
    const i = UPLINK.indexOf(id);
    if (i < 0) return;
    ctx.flash('flPops', 200);
    t.up[i] = 1;
    if (!t.up.every(Boolean)) return;
    t.up = [0, 0, 0];
    t.uplink++;
    const g = ctx.game;
    let value = UPLINK_SCORE;
    if (t.uplink === 1 && !ctx.player.ebLit) ctx.lightExtraBall();
    else value += UPLINK_REPEAT;
    ctx.score(value);
    if (g.mode.id) g.mode.until += UPLINK_MODE_SECONDS;
    if (ctx.mbActive()) g.mb.litShots = [...(ctx.table.rules.multiball?.jackpotShots || [])];
    ctx.dmd('big', { text: 'UPLINK', score: value, vars: {} }, 1600, 7);
    ctx.cue('combo');
    ctx.show('sweepUp', { ms: 900 });
}

function onFirewall(ctx) {
    const p = ctx.player, was = p.lockLit;
    ctx.lightLock();
    if (was || !p.lockLit) return;
    big(ctx, 'dmd_neon_firewall');
    ctx.show('chase', { ms: 1200 });
    ctx.flash('flVault', 400);
    ctx.say('lock_lit');
}

function onEvent(ctx, ev) {
    const t = ctx.player.table;
    if (t.mb && !ctx.mbActive()) { t.mb = false; bank(ctx, 'resetBank'); t.fw = [0, 0, 0]; }
    switch (ev.type) {
    case 'bumper':
        ctx.flash('flPops', 140);
        break;
    case 'drop': {
        const i = FW.indexOf(ev.id);
        if (i >= 0) t.fw[i] = 1;
        break;
    }
    case 'bankReset':
        t.fw = [0, 0, 0];
        break;
    case 'bankDown':
        t.fw = [1, 1, 1];
        if (ev.id === 'firewall') onFirewall(ctx);
        break;
    case 'standup':
        onStandup(ctx, t, ev.id);
        onUplink(ctx, t, ev.id);
        break;
    case 'spinner':
        if (ctx.game.mode.id === 'chase') ctx.score(4000);
        if (++t.spins >= SPIN_RELIGHT) { t.spins = 0; ctx.lightModeStart(); }
        break;
    default:
    }
    checkModeExtraBall(ctx, t);
}

function onShot(ctx, id) {
    const shot = (ctx.table.layout.shots || []).find(s => s.id === id);
    if (SHOT_BONUS[id] && shot && !shot.bonus) ctx.bonus(SHOT_BONUS[id], 1);
    if (SHOT_FLASH[id]) ctx.flash(SHOT_FLASH[id], 250);
    checkModeExtraBall(ctx, ctx.player.table);
}

function lamps(ctx, set) {
    const p = ctx.player, t = p.table, g = ctx.game;
    FW.forEach((id, i) => set(id, p.lockLit ? 'slow' : (t.fw[i] ? 'on' : 'off')));
    LETTERS.forEach((id, i) => set(id, t.letters[i] ? 'on' : 'off'));
    // The bank pays out in modes and multiball: its unlit letters blink slowly then.
    const hot = g.mode.id || g.mb.state !== 'off';
    UPLINK.forEach((id, i) => set(id, t.up[i] ? 'on' : hot ? 'slow' : 'off'));
    set('spinnerLit', g.mode.id === 'chase' ? 'on' : 'off');
}

function toy(g) {
    const s = g.mb.state;
    const state = s === 'wizard' ? 'wizard' : s === 'running' ? 'mb' : g.mode.id ? 'mode' : 'idle';
    const speed = { idle: 0.4, mode: 2, mb: 4, wizard: 6 }[state];
    return { spire: { speed, color: COLORS[state] }, vault: g.balls.locked };
}

export const rules = {
    points: {
        'bumper:*': 3010, 'sling:*': 510, 'spinner:*': 1010, 'rollover:*': 2510, 'standup:*': 5010, 'drop:*': 7510,
        'bankDown:*': 50000, 'hole:*': 25000, 'pathMade:*': 0, default: 1010
    },
    shotPoints: { leftRamp: 50000, rightRamp: 50000, leftOrbit: 30000, rightOrbit: 30000 },
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
    skill: { value: 250000, super: { on: 'hole:dataCore', window: 6, value: 1000000 } },
    ballSave: { seconds: 8, lamp: 'shootAgain' },
    kickback: { device: 'kickbackL', lamp: 'kickbackL', litAtStart: true },
    extraBall: { shot: 'core', lamp: 'extraBall' },
    lock: { hole: 'vault', shot: 'vault', physical: true, lamp: 'lockLit', diverter: { id: 'lockDiverter', shot: 'rightRamp' } },
    multiball: {
        name: 'mode_neon_mb', balls: 3, ballSave: 10, jackpotShots: ['leftRamp', 'rightRamp', 'leftOrbit', 'rightOrbit'],
        jackpotBase: 1000000, jackpotStep: 250000, superAfter: 4, superShot: 'core', superWindow: 20, addABallShot: 'vault'
    },
    modeStart: { shot: 'core', hole: 'dataCore', lamp: 'modeStart' },
    modes: [
        { id: 'hack', name: 'mode_neon_hack', hint: 'dmd_hint_ramps', kind: 'count', targets: ['leftRamp', 'rightRamp'], count: 6, duration: 45, shotValue: 400000, step: 0, completeValue: 3000000 },
        { id: 'blackout', name: 'mode_neon_blackout', hint: 'dmd_hint_lit', kind: 'roving', targets: ['leftOrbit', 'leftRamp', 'core', 'rightRamp', 'rightOrbit'], count: 4, moveEvery: 4, duration: 45, shotValue: 750000, step: 0, completeValue: 4000000, gi: 'dim' },
        { id: 'chase', name: 'mode_neon_chase', hint: 'dmd_hint_orbits', kind: 'count', targets: ['leftOrbit', 'rightOrbit'], count: 5, duration: 40, shotValue: 500000, step: 0, completeValue: 3000000 },
        { id: 'overclock', name: 'mode_neon_overclock', hint: 'dmd_hint_bumpers', kind: 'count', targets: ['bumper:*'], count: 30, duration: 35, shotValue: 100000, step: 0, completeValue: 2500000 }
    ],
    wizard: { id: 'wizard', name: 'mode_neon_wizard', balls: 4, ballSave: 20, jackpotValue: 5000000, step: 1000000, superShot: 'core', superAfter: 6 },
    combos: { window: 3.0, base: 100000 },
    replay: 40000000,
    hooks: {
        init: () => ({ letters: [0, 0, 0, 0], neon: 0, up: [0, 0, 0], uplink: 0, spins: 0, fw: [0, 0, 0], ebModes: false, mb: false }),
        onBallStart(ctx) {
            const t = ctx.player.table;
            t.mb = false; t.fw = [0, 0, 0];
            bank(ctx, 'resetBank');
        },
        onLock(ctx, n) {
            ctx.flash('flVault', 400);
            if (n < 3) { bank(ctx, 'resetBank'); ctx.player.table.fw = [0, 0, 0]; }
        },
        onMultiball(ctx) {
            ctx.player.table.mb = true;
            bank(ctx, 'dropBank');
            ctx.flash('flVault', 500);
            ctx.say('multiball');
        },
        onEvent, onShot, lamps, toy
    }
};

export default rules;
