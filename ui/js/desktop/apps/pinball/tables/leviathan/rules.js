// Deep Sea Leviathan: rule pack (plan 6.2). Pearl strikes light the virtual lock on the tentacle ramp, three locks start
// ABYSS MULTIBALL, the five sonar targets light Deep Grab (the magnet throws a ball at the ship), the ship starts modes.
// Pure and deterministic: randomness comes from ctx.rng, timers from hooks.onTick (ctx.now), all output goes through ctx.
const SONAR = ['sonar1', 'sonar2', 'sonar3', 'sonar4', 'sonar5'];
const SHOTS = ['leftOrbit', 'current', 'pearl', 'ship', 'tentacle', 'rightOrbit'];
const SHOT_BONUS = { current: 'ramps', tentacle: 'ramps', leftOrbit: 'orbits', rightOrbit: 'orbits' };
const SHOT_FLASH = { pearl: 'flPearl', ship: 'flShip', tentacle: 'flTentacle', current: 'flCurrent' };
const COLORS = { idle: '#43e3d5', lock: '#7fd3ff', mode: '#dcad63', grab: '#ffd24a', mb: '#ff5a3c', wizard: '#ffffff' };
const GRAB_WINDOW = 10, GRAB_HOLD = 1.5, ASSIST_HOLD = 0.6, ASSIST_REARM = 0.6, THROW_SPEED = 1.4, CAPTURE_WINDOW = 2;
const GRAB_VALUE = 1000000, SPIN_RELIGHT = 10, FALLBACK_ANGLE = 70 * Math.PI / 180;

const big = (ctx, key, ms = 1500) => ctx.dmd('big', { text: key, vars: {} }, ms, 7);
const magnet = (ctx, mode, extra = {}) => ctx.cmd({ type: 'magnet', id: 'abyss', mode, ...extra });

// Math angle (0 = +x, pi/2 = up the table) from the abyss magnet to the ship scoop, from the layout when it has both.
function shipAngle(ctx) {
    const dv = ctx.table.layout.devices || [];
    const a = dv.find(d => d.id === 'abyss'), s = dv.find(d => d.id === 'shipScoop');
    return a && s ? Math.atan2(s.y - a.y, s.x - a.x) : FALLBACK_ANGLE;
}

function checkModeExtraBall(ctx, t) {
    if (ctx.player.modesCompleted.length < 2) { t.ebModes = false; return; }
    if (!t.ebModes) { t.ebModes = true; ctx.lightExtraBall(); }
}

function resetGrab(t) { t.grabLit = false; t.window = 0; t.hold = null; t.capture = 0; }

function onSonar(ctx, t, id) {
    const i = SONAR.indexOf(id);
    if (i < 0) return;
    t.sonar[i] = 1;
    if (!t.sonar.every(Boolean)) return;
    t.sonar = [0, 0, 0, 0, 0];
    t.sonars++;
    ctx.score(100000);
    ctx.lightKickback();
    ctx.lightModeStart();
    t.grabLit = true; t.window = ctx.now + GRAB_WINDOW; t.hold = null;
    magnet(ctx, 'grab');
    big(ctx, 'dmd_leviathan_sonar');
    ctx.show('radial', { ms: 1200 });
    ctx.say('lock_lit');
}

// Pearl strikes: 3 light the lock (2 once it has been lit before); a pearl that rides to the top counts one more.
function onPearl(ctx, t, top) {
    const p = ctx.player;
    ctx.flash('flPearl', 300);
    if (ctx.mbActive() || p.lockLit || p.locks >= 3) return;
    if (top) {
        big(ctx, 'dmd_leviathan_pearl');
        ctx.show('sparkle', { ms: 900 });
    }
    t.pearl += 1;
    const need = t.lit > 0 ? 2 : 3;
    if (t.pearl < need) return;
    t.pearl = 0;
    ctx.lightLock();
    if (p.lockLit) { t.lit++; ctx.flash('flPearl', 500); ctx.say('lock_lit'); }
}

function onMagnet(ctx, t, ev) {
    if (ev.state === 'held') {
        if (t.grabLit) { t.hold = { at: ctx.now + GRAB_HOLD, kind: 'deep' }; big(ctx, 'dmd_leviathan_grab', 1200); ctx.show('ring', { ms: 1200 }); }
        else if (t.assist) t.hold = { at: ctx.now + ASSIST_HOLD, kind: 'assist' };
    } else if (ev.state === 'release') {
        // The coils dropped (tilt) or the magnet was freed by a command: forget the hold.
        if (t.hold && t.hold.kind === 'deep') resetGrab(t);
        t.hold = null;
    }
}

function onEvent(ctx, ev) {
    const t = ctx.player.table, g = ctx.game;
    if (t.mb && !ctx.mbActive()) t.mb = false;
    switch (ev.type) {
    case 'bumper':
        ctx.flash('flCoral', 140);
        break;
    case 'captive':
        if (ev.id === 'pearl') onPearl(ctx, t, false);
        break;
    case 'trigger':
        if (ev.id === 'pearlTop') onPearl(ctx, t, true);
        break;
    case 'standup':
        onSonar(ctx, t, ev.id);
        break;
    case 'spinner':
        if (g.mode.id === 'tsunami' || g.mb.state === 'running') ctx.score(4000);
        if (++t.spins >= SPIN_RELIGHT) { t.spins = 0; ctx.lightModeStart(); }
        break;
    case 'magnet':
        onMagnet(ctx, t, ev);
        break;
    case 'hole':
        if (ev.id === 'shipScoop' && t.capture > ctx.now) {
            t.capture = 0;
            const v = ctx.score(GRAB_VALUE);
            ctx.dmd('big', { text: 'dmd_leviathan_grab', score: v, vars: {} }, 2000, 7);
            ctx.cue('jackpot'); ctx.show('radial', { ms: 1200 }); ctx.shake(0.002);
        }
        break;
    default:
    }
    checkModeExtraBall(ctx, t);
}

// Timers: the deep-grab window and hold, the 2 s capture window and the treasure magnet assist.
function onTick(ctx) {
    const t = ctx.player.table, now = ctx.now, g = ctx.game;
    if (t.hold && now >= t.hold.at) {
        const deep = t.hold.kind === 'deep';
        magnet(ctx, 'throw', { angle: shipAngle(ctx), speed: THROW_SPEED });
        t.hold = null;
        if (deep) { t.grabLit = false; t.window = 0; t.capture = now + CAPTURE_WINDOW; } else t.rearm = now + ASSIST_REARM;
    }
    if (t.grabLit && !t.hold && t.window && now >= t.window) { resetGrab(t); magnet(ctx, 'off'); }
    if (t.capture && now > t.capture) t.capture = 0;
    const want = g.mode.id === 'treasure' && !g.select;
    if (want && !t.assist && !t.hold && !t.grabLit) { t.assist = true; magnet(ctx, 'grab'); }
    else if (!want && t.assist) { t.assist = false; if (!t.hold && !t.grabLit) magnet(ctx, 'off'); }
    else if (want && t.assist && t.rearm && now >= t.rearm) { t.rearm = 0; if (!t.grabLit) magnet(ctx, 'grab'); }
}

function onShot(ctx, id) {
    const shot = (ctx.table.layout.shots || []).find(s => s.id === id);
    if (SHOT_BONUS[id] && shot && !shot.bonus) ctx.bonus(SHOT_BONUS[id], 1);
    if (SHOT_FLASH[id]) ctx.flash(SHOT_FLASH[id], 250);
    checkModeExtraBall(ctx, ctx.player.table);
}

function lamps(ctx, set) {
    const p = ctx.player, t = p.table, g = ctx.game, mb = g.mb.state;
    SONAR.forEach((id, i) => set(id, t.sonar[i] ? 'on' : 'off'));
    [1, 2, 3].forEach(n => set('pearl' + n, p.lockLit ? 'slow' : (t.pearl >= n ? 'on' : 'off')));
    set('grab', t.hold && t.hold.kind === 'deep' ? 'fast' : t.grabLit ? 'slow' : 'off');
    set('spinnerLit', g.mode.id === 'tsunami' || mb === 'running' ? 'on' : 'off');
    const eye = mb !== 'off' ? 'fast' : g.mode.id ? 'on' : p.lockLit ? 'slow' : 'off';
    set('eyeL', eye); set('eyeR', eye);
}

// Eye colour and openness per state; the jaw opens in multiball and the wizard.
function toy(g) {
    const t = g.players[g.turn].table, s = g.mb.state;
    if (s === 'wizard') return { eye: { color: COLORS.wizard, open: 1 }, jaw: 1 };
    if (s === 'running') return { eye: { color: COLORS.mb, open: 1 }, jaw: 0.6 };
    if (t.hold && t.hold.kind === 'deep') return { eye: { color: COLORS.grab, open: 0.9 }, jaw: 0 };
    if (g.mode.id) return { eye: { color: COLORS.mode, open: 0.7 }, jaw: 0 };
    if (g.players[g.turn].lockLit) return { eye: { color: COLORS.lock, open: 0.55 }, jaw: 0 };
    return { eye: { color: COLORS.idle, open: 0.3 }, jaw: 0 };
}

export const rules = {
    points: {
        'bumper:*': 3010, 'sling:*': 510, 'spinner:*': 1010, 'rollover:*': 2510, 'standup:*': 5010, 'captive:*': 5010,
        'hole:*': 25000, 'pathMade:*': 0, default: 1010
    },
    shotPoints: { current: 50000, tentacle: 50000, leftOrbit: 30000, rightOrbit: 30000, pearl: 10000 },
    bonusFrom: { 'standup:*': 'targets', 'captive:*': 'targets', 'bumper:*': 'bumpers', 'spinner:*': 'spins' },
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
    skill: { value: 250000, super: { on: 'hole:shipScoop', window: 6, value: 1000000 } },
    ballSave: { seconds: 8, lamp: 'shootAgain' },
    kickback: { device: 'kickbackR', lamp: 'kickbackR', litAtStart: true },
    extraBall: { shot: 'ship', lamp: 'extraBall' },
    lock: { shot: 'tentacle', physical: false, lamp: 'lockLit', diverter: null },
    multiball: {
        name: 'mode_leviathan_mb', balls: 3, ballSave: 10, jackpotShots: ['current', 'tentacle', 'leftOrbit', 'rightOrbit'],
        jackpotBase: 1000000, jackpotStep: 250000, superAfter: 4, superShot: 'ship', superWindow: 20, addABallShot: 'pearl'
    },
    modeStart: { shot: 'ship', hole: 'shipScoop', lamp: 'modeStart' },
    modes: [
        { id: 'tsunami', name: 'mode_leviathan_tsunami', hint: 'dmd_hint_ramps', kind: 'count', targets: ['current', 'tentacle'], count: 5, duration: 45, shotValue: 400000, step: 0, completeValue: 3000000 },
        { id: 'sonar', name: 'mode_leviathan_sonar', hint: 'dmd_hint_lit', kind: 'roving', targets: SONAR.map(id => 'standup:' + id), count: 5, moveEvery: 4, duration: 40, shotValue: 500000, step: 0, completeValue: 3500000 },
        { id: 'treasure', name: 'mode_leviathan_treasure', hint: 'dmd_hint_scoop', kind: 'count', targets: ['ship'], count: 3, duration: 50, shotValue: 750000, step: 0, completeValue: 4000000 },
        { id: 'kraken', name: 'mode_leviathan_kraken', hint: 'dmd_hint_lit', kind: 'roving', targets: SHOTS, count: 4, moveEvery: 4, duration: 45, shotValue: 750000, step: 0, completeValue: 4000000 }
    ],
    wizard: { id: 'wizard', name: 'mode_leviathan_wizard', balls: 4, ballSave: 20, jackpotValue: 5000000, step: 1000000, superShot: 'ship', superAfter: 6 },
    combos: { window: 3.0, base: 100000 },
    replay: 40000000,
    hooks: {
        init: () => ({
            sonar: [0, 0, 0, 0, 0], sonars: 0, pearl: 0, lit: 0, spins: 0, ebModes: false, mb: false,
            grabLit: false, window: 0, hold: null, capture: 0, assist: false, rearm: 0
        }),
        onBallStart(ctx) {
            const t = ctx.player.table;
            t.pearl = 0; t.mb = false; t.assist = false; t.rearm = 0;
            resetGrab(t);
            magnet(ctx, 'off');
        },
        onLock(ctx) { ctx.flash('flPearl', 400); ctx.player.table.pearl = 0; },
        onMultiball(ctx) {
            ctx.player.table.mb = true;
            ctx.flash('flShip', 500);
            ctx.say('multiball');
        },
        onEvent, onShot, onTick, lamps, toy
    }
};

export default rules;
