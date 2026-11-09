// AuraPinball sound effects: event/cue -> recipe mapping (pure), 24-voice limiter (pure), and the player that
// runs recipes through the engine. The pure helpers never touch AudioContext, so Node tests import them freely.
import { RECIPES, makeKit } from './recipes.js';

export const MAX_VOICES = 24;
export const MAX_ROLLS = 4;
export const PAN_RANGE = 0.26;
export const IMPACT_MIN_SPEED = 0.15;
const ROLL_GAIN = 1.0;
const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

export function panFor(x) { return Number.isFinite(x) ? clamp(x / PAN_RANGE, -1, 1) * 0.8 : 0; }

function hash(text) { let h = 2166136261; for (let i = 0; i < text.length; i++) h = Math.imul(h ^ text.charCodeAt(i), 16777619); return (h >>> 0) / 4294967296; }
// +-3 % deterministic pitch spread so repeated hits do not machine-gun.
function spread(ev) { return 1 + (hash(String(ev.id ?? '') + ':' + (ev.tick ?? 0)) - .5) * .06; }

// Events that deliberately make no sound (scoring/rules/visuals handle them, or they are covered by another event).
export const SILENT_EVENTS = Object.freeze(['bankDown', 'pathEnter', 'pathMade', 'pathFail', 'search', 'searchFail']);

const IMPACT_RECIPE = { wood: 'impactWood', metal: 'impactMetal', plastic: 'impactPlastic', rubber: 'impactRubber', flipper: 'impactFlipper', target: 'impactTarget', ball: 'impactBall' };

function side(ev) {
    if (Number.isFinite(ev.x)) return panFor(ev.x);
    const id = String(ev.id ?? '');
    return /left/i.test(id) ? -.45 : /right/i.test(id) ? .45 : 0;
}

// Pure: physics event -> { recipe, gain, pan, rate } or null.
export function soundFor(ev) {
    if (!ev || typeof ev.type !== 'string') return null;
    const pan = side(ev), rate = spread(ev), s = (recipe, gain, r = rate, p = pan) => ({ recipe, gain: clamp(gain, 0, 1), pan: p, rate: r });
    switch (ev.type) {
        case 'flipper': return ev.up ? s('flipperUp', 1) : s('flipperDown', .7);
        case 'launch': return s('plungerRelease', .55 + clamp((ev.speed ?? 4) / 12, 0, .45));
        case 'drain': return s('drain', 1, 1, 0);
        case 'bumper': return s('bumper', .95, (.92 + Math.floor(hash(String(ev.id ?? '')) * 5) * .04) * rate);
        case 'sling': return s('sling', .95);
        case 'drop': return s('drop', .9);
        case 'bankReset': return s('bankReset', .9);
        case 'standup': return s('standup', .8);
        case 'spinner': return s('spinnerTick', .6, clamp(.92 + (ev.rate ?? 0) / 60, .92, 1.25) * rate);
        case 'rollover': case 'trigger': return s('rollover', .6);
        case 'gate': case 'pathSwitch': return s('gate', ev.type === 'gate' ? .55 : .35);
        case 'hole': return s('scoop', 1);
        case 'eject': return s('eject', .9);
        case 'kickback': return s('kickback', 1);
        case 'magnet':
            if (ev.state === 'grab') return s('magnetOn', .8);
            if (ev.state === 'release') return s('magnetOn', .5, .7);
            if (ev.state === 'throw') return s('eject', .9);
            return null;
        case 'captive': return s('impactBall', .9);
        case 'shooter': return s('clack', .35);
        case 'clack': return s('clack', ev.speed === undefined ? .7 : clamp(ev.speed / 3, .15, 1));
        case 'impact': {
            const v = ev.speed ?? ev.vn ?? 1;
            if (!(v >= IMPACT_MIN_SPEED)) return null;
            return s(IMPACT_RECIPE[ev.material] || 'impactMetal', clamp(v / 3, .05, 1));
        }
        default: return null;
    }
}

// Cue accents played by the SFX layer; the music layer adds melodic stingers on top where its song defines them.
const CUE_TABLE = {
    jackpot: ['jackpotHit', 1, 1], superJackpot: ['jackpotHit', 1, .89], lock: ['lockThunk', 1, 1], lockLit: ['bellHigh', .8, 1],
    multiball: ['riser', 1, 1], extraBall: ['bell', .9, 1.25], replay: ['bell', .9, 1.5], modeStart: ['riser', .7, 1.2],
    modeComplete: ['bell', .9, 1], skillShot: ['bellHigh', .9, 1.19], ballSaved: ['bellHigh', .8, 1.5], drain: ['drain', 1, 1],
    gameOver: ['descend', 1, 1], highScore: ['bell', .9, 1.19], tiltWarning: ['tiltWarnBuzz', .9, 1], tilt: ['tiltBuzzer', 1, 1],
    combo: ['bellHigh', .6, 1.33], bonusTick: ['tick', .6, 1], bonusTotal: ['bonusTotal', 1, 1], match: ['bell', .8, .89],
    knocker: ['knocker', 1, 1], startGame: ['riser', .6, .8], shootAgain: ['bellHigh', .8, 1.26], wizard: ['riser', 1, .75]
};
export function cueSoundFor(name) { const c = CUE_TABLE[name]; return c ? { recipe: c[0], gain: c[1], pan: 0, rate: c[2] } : null; }

// Pure: which continuous roll texture a ball makes, or null when it is silent.
export function rollKind(ball) {
    if (!ball || ball.mode === 'held' || ball.mode === 'shooter' || ball.mode === 'captive' || ball.hidden) return null;
    if (ball.mode === 'path') return ball.pathKind === 'wire' ? 'wire' : ball.pathKind === 'tube' ? 'tube' : 'plastic';
    return 'field';
}
export function rollLevel(speed) { return speed > .05 ? Math.pow(clamp(speed / 2.5, 0, 1), 1.5) * .5 : 0; }

// Pure: voice stealing. acquire() returns a handle, or null when the new voice itself is the lowest priority.
export function createVoiceLimiter(max = MAX_VOICES) {
    let voices = [], serial = 0;
    const purge = now => { voices = voices.filter(v => v.end > now); };
    return {
        acquire(priority, end, now = 0, stop = null) {
            purge(now);
            if (voices.length >= max) {
                let victim = voices[0];
                for (const v of voices) if (v.priority < victim.priority) victim = v;
                if (victim.priority > priority) return null;
                voices.splice(voices.indexOf(victim), 1);
                if (victim.stop) victim.stop();
            }
            const handle = { id: ++serial, priority, end, stop };
            voices.push(handle);
            return handle;
        },
        size(now = 0) { purge(now); return voices.length; },
        clear() { voices = []; }
    };
}

export function createSfx(engine) {
    const limiter = createVoiceLimiter(MAX_VOICES), kit = makeKit(engine);
    let slots = null, magnet = null, plungerStep = 0, dead = false;

    function running() { return !dead && engine.ctx && engine.ctx.state === 'running' && engine.buses; }
    function play(snd, delay = 0) {
        if (!snd || !running()) return false;
        const def = RECIPES[snd.recipe];
        if (!def) return false;
        const c = engine.ctx, t = c.currentTime + .004 + delay;
        const vg = c.createGain();
        const handle = limiter.acquire(def.priority, t + def.dur, c.currentTime, () => { try { vg.gain.cancelScheduledValues(c.currentTime); vg.gain.setTargetAtTime(0, c.currentTime, .01); } catch { /* ignore */ } });
        if (!handle) return false;
        vg.gain.value = snd.gain;
        let tail = vg;
        if (c.createStereoPanner) { const p = c.createStereoPanner(); p.pan.value = clamp(snd.pan, -1, 1); vg.connect(p); tail = p; }
        tail.connect(engine.buses.effects);
        if (def.wet > 0 && engine.reverbSend) { const send = c.createGain(); send.gain.value = def.wet; tail.connect(send); send.connect(engine.reverbSend); }
        def.run(kit, t, snd.rate, vg);
        if (def.duck && engine.duckMusic) engine.duckMusic(def.duck * (.5 + .5 * snd.gain), def.dur * .5);
        return true;
    }

    function event(ev) {
        if (ev && ev.type === 'magnet') {
            if (ev.state === 'grab' || ev.state === 'held') setMagnet(true);
            else if (ev.state === 'release' || ev.state === 'throw') setMagnet(false);
        }
        return play(soundFor(ev));
    }
    function cue(name) { return play(cueSoundFor(name)); }

    // Plunger ratchet: one click per 1/12 of travel while pulling (pull 0..1); resets on release.
    function plunger(pull) {
        const step = Math.floor(clamp(pull || 0, 0, 1) * 12);
        if (step > plungerStep) play({ recipe: 'plungerRatchet', gain: .5 + step / 24, pan: 0, rate: .9 + step * .02 });
        plungerStep = step;
    }

    // ---- continuous rolling: looped recordings (samples.js LOOPS), max 4 voices ----
    // Per voice three looping sources (hard roll, soft roll, metal scrape) behind their own filters; the ball's surface
    // kind picks the mix, its speed drives gain and playback rate. Everything moves with setTargetAtTime (no zipper noise).
    function buildSlots() {
        const c = engine.ctx, hardB = engine.loop('hard'), softB = engine.loop('soft'), scrapeB = engine.loop('scrape');
        if (!hardB || !softB || !scrapeB) return null; // samples still loading: try again on the next roll() call
        const filt = (type, freq, q = .7) => { const f = c.createBiquadFilter(); f.type = type; f.frequency.value = freq; f.Q.value = q; return f; };
        const gain = v => { const g = c.createGain(); g.gain.value = v; return g; };
        const loopSrc = buf => { const s = c.createBufferSource(); s.buffer = buf; s.loop = true; s.start(0, Math.random() * buf.duration); return s; };
        return Array.from({ length: MAX_ROLLS }, () => {
            const level = gain(0), lfo = c.createOscillator(), depth = gain(0), am = gain(1);
            const hard = loopSrc(hardB), hardLp = filt('lowpass', 5000), hardHp = filt('highpass', 40), hardG = gain(0);
            hard.connect(hardLp); hardLp.connect(hardHp); hardHp.connect(hardG); hardG.connect(level);
            const soft = loopSrc(softB), softBp = filt('bandpass', 900, 2), softG = gain(0);
            soft.connect(softBp); softBp.connect(softG); softG.connect(level);
            const scrape = loopSrc(scrapeB), scrapeHp = filt('highpass', 1500), scrapeG = gain(0);
            scrape.connect(scrapeHp); scrapeHp.connect(am); am.connect(scrapeG); scrapeG.connect(level);
            lfo.type = 'square'; lfo.frequency.value = 30; lfo.connect(depth); depth.connect(am.gain); lfo.start();
            let tail = level;
            const pan = c.createStereoPanner ? c.createStereoPanner() : null;
            if (pan) { level.connect(pan); tail = pan; }
            tail.connect(engine.buses.effects);
            return { srcs: [hard, soft, scrape], lfo, level, pan, hardLp, hardHp, hardG, softBp, softG, scrapeHp, scrapeG, am, depth, kind: '' };
        });
    }
    // Per surface kind: mix gains, filter settings and the playback-rate range over ball speed.
    const ROLL_KINDS = {
        field: { hard: 1, soft: 0, scrape: 0, lp: 5500, hp: 40, rate: [.7, .75] },
        plastic: { hard: .7, soft: 1.5, scrape: 0, lp: 2600, hp: 250, bp: 1500, q: 1.1, rate: [.9, .5] },
        wire: { hard: .8, soft: 0, scrape: 2.2, lp: 9000, hp: 2000, scrapeHp: 1400, am: 1, rate: [.9, .5] },
        tube: { hard: 1.15, soft: 0, scrape: 0, lp: 650, hp: 30, rate: [.6, .5] }
    };
    function shape(slot, kind, speed, t) {
        const p = ROLL_KINDS[kind] || ROLL_KINDS.field, k = clamp(speed / 3, 0, 1), tc = .05;
        slot.kind = kind;
        slot.hardLp.frequency.setTargetAtTime(p.lp, t, tc); slot.hardHp.frequency.setTargetAtTime(p.hp, t, tc);
        slot.softBp.frequency.setTargetAtTime(p.bp || 900, t, tc); slot.softBp.Q.setTargetAtTime(p.q || 2, t, tc);
        slot.scrapeHp.frequency.setTargetAtTime(p.scrapeHp || 1500, t, tc);
        slot.hardG.gain.setTargetAtTime(p.hard, t, tc); slot.softG.gain.setTargetAtTime(p.soft, t, tc); slot.scrapeG.gain.setTargetAtTime(p.scrape, t, tc);
        slot.depth.gain.setTargetAtTime(p.am ? .42 : 0, t, tc); slot.am.gain.setTargetAtTime(p.am ? .58 : 1, t, tc);
        slot.lfo.frequency.setTargetAtTime(p.am ? 26 + 30 * k : 30, t, tc);
        const rate = p.rate[0] + p.rate[1] * k;
        for (const s of slot.srcs) s.playbackRate.setTargetAtTime(rate, t, .06);
    }
    function roll(balls) {
        if (!running()) return;
        if (!slots) { slots = buildSlots(); if (!slots) return; }
        const t = engine.ctx.currentTime, active = [];
        for (const b of balls || []) { const kind = rollKind(b), lvl = kind ? rollLevel(b.speed || 0) : 0; if (lvl > 0) active.push({ b, kind, lvl }); }
        active.sort((a, b) => b.lvl - a.lvl);
        slots.forEach((slot, i) => {
            const a = active[i];
            if (!a) { slot.level.gain.setTargetAtTime(0, t, .08); return; }
            shape(slot, a.kind, a.b.speed, t);
            slot.level.gain.setTargetAtTime(a.lvl * ROLL_GAIN, t, .06);
            if (slot.pan) slot.pan.pan.setTargetAtTime(panFor(a.b.x), t, .05);
        });
    }

    // ---- magnet hum 60/120 Hz ----
    function setMagnet(on) {
        if (!running()) return;
        const c = engine.ctx, t = c.currentTime;
        if (on && !magnet) {
            const g = c.createGain(), f = c.createBiquadFilter(), a = c.createOscillator(), b = c.createOscillator(), bg = c.createGain();
            g.gain.value = 0; f.type = 'lowpass'; f.frequency.value = 400; a.type = 'sine'; a.frequency.value = 60; b.type = 'sawtooth'; b.frequency.value = 120; bg.gain.value = .25;
            a.connect(f); b.connect(bg); bg.connect(f); f.connect(g); g.connect(engine.buses.effects); a.start(); b.start();
            g.gain.setTargetAtTime(.32, t, .08);
            magnet = { g, a, b };
        } else if (!on && magnet) {
            const m = magnet; magnet = null;
            m.g.gain.setTargetAtTime(0, t, .06); m.a.stop(t + .4); m.b.stop(t + .4);
        }
    }

    function stopAll() {
        if (slots) { for (const s of slots) { try { for (const src of s.srcs) src.stop(); s.lfo.stop(); s.level.disconnect(); } catch { /* ignore */ } } slots = null; }
        if (magnet) { try { magnet.a.stop(); magnet.b.stop(); magnet.g.disconnect(); } catch { /* ignore */ } magnet = null; }
        limiter.clear();
    }
    return {
        event, cue, play, roll, plunger, magnet: setMagnet, stopAll,
        voices: () => engine.ctx ? limiter.size(engine.ctx.currentTime) : 0,
        dispose() { dead = true; stopAll(); }
    };
}
