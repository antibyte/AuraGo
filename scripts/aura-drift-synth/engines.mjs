// Engine loops: 8 cars x 4 layers, 2 s mono, seamless. Each engine is a train of firing events (one pulse per
// cylinder, with its own amplitude and timing error) pushed through exhaust-pipe resonances; diesel knock, intake
// noise, valve ticks and turbo whistle ride on the same firing times. Everything is built circularly so the loop closes.
import { SR, mk, rng, seedOf, filter, addWrap, softClip, dcBlock, white, clamp, TAU, loopHz, peak } from './dsp.mjs';

export const ENGINE_LAYERS = { idle: 1000, low: 2600, mid: 4600, high: 6600 };
export const CAR_IDS = ['vektor', 'kobalt', 'lumen', 'tundra', 'softeis', 'maehkoenig', 'couch', 'bertha'];
const N = 2 * SR;

const tile3 = x => { const o = mk(3 * N); o.set(x, 0); o.set(x, N); o.set(x, 2 * N); return o; };
const middle = x => x.slice(N, 2 * N);
const circular = (x, fn) => middle(fn(tile3(x)));
const pipeRes = (f1, count, q, tilt, every = 2) => Array.from({ length: count }, (_, m) => [f1 * (1 + every * m), Math.max(1.6, q * 0.45), Math.pow(tilt, m)]);

const SPECS = {
    // wedge turbo: crisp four-cylinder with a whistling turbo
    vektor: {
        stroke: 4, order: [0, 2, 3, 1], bank: [0, 0, 0, 0], cylAmp: [1, 0.95, 1.05, 0.98], timing: [0, 0.012, -0.01, 0.008],
        jitter: 0.012, ampVar: 0.05, rise: 0.16, decay: 0.55, pipes: [pipeRes(108, 8, 7, 0.82)], bodyF: 150, body: 0.55,
        knock: 0.05, knockBand: [2200, 2], intake: 0.35, tick: 0.12, turbo: { ratio: 5.5, gain: 0.5, start: 1800 }, drive: 1.5, lp: 7500
    },
    // GT six: smooth inline-six with a howl, no turbo
    kobalt: {
        stroke: 4, order: [0, 4, 2, 5, 1, 3], bank: [0, 0, 0, 0, 0, 0], cylAmp: [1, 1.01, 0.99, 1, 0.98, 1.02], timing: [0, 0.004, -0.004, 0.003, -0.003, 0.002],
        jitter: 0.006, ampVar: 0.03, rise: 0.25, decay: 0.75, pipes: [pipeRes(92, 9, 9, 0.86)], bodyF: 130, body: 0.5,
        knock: 0.03, knockBand: [1800, 2], intake: 0.65, tick: 0.08, turbo: null, drive: 1.2, lp: 9000
    },
    // muscle V8: cross-plane firing order over two banks gives the lope
    lumen: {
        stroke: 4, order: [0, 7, 3, 2, 5, 4, 6, 1], bank: [0, 1, 0, 1, 1, 0, 0, 1], cylAmp: [1, 1.08, 0.93, 1.04, 0.97, 1.1, 0.92, 1.0], timing: [0, 0.03, -0.02, 0.025, -0.03, 0.01, -0.01, 0.02],
        jitter: 0.02, ampVar: 0.11, rise: 0.4, decay: 1.5, pipes: [pipeRes(72, 7, 5, 0.8), pipeRes(81, 7, 5, 0.8)], bodyF: 110, body: 1.1,
        knock: 0.08, knockBand: [1100, 1.4], intake: 0.3, tick: 0.1, turbo: null, drive: 1.8, lp: 5800, idleLope: 2.2
    },
    // rally boxer: unequal-length headers, burble, little turbo
    tundra: {
        stroke: 4, order: [0, 2, 1, 3], bank: [0, 1, 0, 1], cylAmp: [1, 1.12, 0.92, 1.05], timing: [0, 0.02, -0.015, 0.012],
        jitter: 0.015, ampVar: 0.08, rise: 0.22, decay: 0.8, pipes: [pipeRes(64, 8, 6, 0.8), pipeRes(98, 8, 6, 0.8)], delays: [0.0028, 0], bodyF: 120, body: 0.85,
        knock: 0.1, knockBand: [1500, 1.6], intake: 0.3, tick: 0.14, turbo: { ratio: 4.2, gain: 0.22, start: 2500 }, drive: 2.0, lp: 6800, idleLope: 1.6
    },
    // van diesel: sharp combustion knock, loose clatter
    softeis: {
        stroke: 4, order: [0, 2, 3, 1], bank: [0, 0, 0, 0], cylAmp: [1, 0.92, 1.08, 0.96], timing: [0, 0.02, -0.02, 0.015],
        jitter: 0.02, ampVar: 0.1, rise: 0.06, decay: 1.1, pipes: [pipeRes(68, 6, 4, 0.7)], bodyF: 140, body: 0.6,
        knock: 0.95, knockBand: [1700, 1.1], intake: 0.15, tick: 0.55, turbo: { ratio: 3.4, gain: 0.12, start: 3000 }, drive: 1.3, lp: 4800, idleLope: 1.4
    },
    // mower two-stroke: one firing per revolution, expansion chamber ring, misfires at idle
    maehkoenig: {
        stroke: 2, order: [0], bank: [0], cylAmp: [1], timing: [0], jitter: 0.04, ampVar: 0.25, rise: 0.05, decay: 0.45,
        pipes: [[[620, 4.5, 1], [1250, 5, 0.7], [1900, 6, 0.55], [2900, 6, 0.4], [4100, 5, 0.3], [260, 3, 0.55]]], bodyF: 220, body: 0.4,
        knock: 0.28, knockBand: [3400, 1.2], intake: 0.5, tick: 0.2, turbo: null, drive: 2.4, lp: 8500, misfireIdle: 0.28, blade: 0.35
    },
    // truck: big six-cylinder diesel, heavy pulse, turbo whoosh
    bertha: {
        stroke: 4, order: [0, 4, 2, 5, 1, 3], bank: [0, 0, 0, 0, 0, 0], cylAmp: [1, 0.95, 1.05, 0.97, 1.03, 0.99], timing: [0, 0.015, -0.015, 0.012, -0.012, 0.008],
        jitter: 0.015, ampVar: 0.09, rise: 0.12, decay: 2.4, pipes: [pipeRes(48, 8, 4.5, 0.78)], bodyF: 95, body: 1.3,
        knock: 0.65, knockBand: [950, 1.0], intake: 0.22, tick: 0.3, turbo: { ratio: 2.6, gain: 0.3, start: 2000 }, drive: 1.7, lp: 3800, idleLope: 1.5
    }
};

function renderCombustion(carId, rpmNominal) {
    const spec = SPECS[carId], r = rng(seedOf(`engine:${carId}:${rpmNominal}`));
    const four = spec.stroke === 4;
    const cycles = four ? Math.round(rpmNominal / 60) : Math.round(rpmNominal / 30);
    const perCycle = spec.order.length, Nf = cycles * perCycle, T = N / Nf;
    const rpm = four ? cycles * 60 : cycles * 30;
    const banks = spec.pipes.length, exc = Array.from({ length: banks }, () => mk(N));
    const body = mk(N), knock = mk(N), tick = mk(N), env = mk(N);
    const lope = 1 + (spec.idleLope ? (spec.idleLope - 1) * clamp(1 - rpm / 3200, 0, 1) : 0);
    const wob = [loopHz(N, 0.5), loopHz(N, 1.0), loopHz(N, 1.5)], wph = [r() * TAU, r() * TAU, r() * TAU];
    const pulseLen = Math.round(0.012 * SR), shape = (t, rise, dec) => (Math.exp(-t / dec) - Math.exp(-t / rise));
    for (let k = 0; k < Nf; k++) {
        const c = spec.order[k % perCycle];
        const tk = (k + spec.timing[c] * lope + r.gauss() * spec.jitter * lope * 1.2) * T;
        const w = 1 + 0.06 * Math.sin(TAU * wob[0] * tk / SR * 1 + wph[0]) + 0.04 * Math.sin(TAU * wob[1] * tk / SR + wph[1]);
        let amp = spec.cylAmp[c] * (1 + r.gauss() * (spec.ampVar * 1.7 + 0.04) * lope) * w;
        if (spec.misfireIdle && r() < spec.misfireIdle * clamp(1 - rpm / 3000, 0, 1)) amp *= 0.12;
        amp = Math.max(0.05, amp);
        const pos = tk + (spec.delays ? spec.delays[spec.bank[c]] * SR : 0);
        const i0 = Math.floor(pos), frac = pos - i0, bank = spec.bank[c];
        const soft = 1 + 2.2 * clamp(1 - rpm / 2200, 0, 1), rise = spec.rise * soft * 1e-3 * SR, dec = spec.decay * soft * 1e-3 * SR;
        const p = mk(pulseLen), pb = mk(Math.round(0.03 * SR));
        for (let j = 0; j < pulseLen; j++) p[j] = shape(j - frac, rise, dec) * amp;
        addWrap(exc[bank], p, i0);
        for (let j = 0; j < pb.length; j++) pb[j] = shape(j - frac, rise * 8, dec * 14) * amp;
        addWrap(body, pb, i0);
        // diesel knock and valve ticks: short noise bursts on the firing time
        const kl = Math.round(0.009 * SR), kb = mk(kl);
        for (let j = 0; j < kl; j++) kb[j] = (r() * 2 - 1) * Math.exp(-j / (0.0022 * SR)) * amp;
        addWrap(knock, kb, i0 + Math.round(0.0004 * SR));
        const tl = Math.round(0.003 * SR), tb = mk(tl);
        for (let j = 0; j < tl; j++) tb[j] = (r() * 2 - 1) * Math.exp(-j / (0.0006 * SR));
        addWrap(tick, tb, i0 + Math.round((0.008 + 0.012 * r()) * SR), 1);
        const el = Math.round(0.02 * SR); for (let j = 0; j < el; j++) env[(i0 + j) % N] += amp * Math.exp(-j / (0.006 * SR));
    }
    // exhaust: parallel resonators per bank
    const pipeOut = mk(N);
    spec.pipes.forEach((res, b) => {
        const t = tile3(exc[b]), acc = mk(3 * N);
        for (const [f, q, g] of res) {
            const y = filter(t, 'bp', f, q), gg = g * Math.sqrt(q / 5);
            for (let i = 0; i < acc.length; i++) acc[i] += y[i] * gg;
        }
        const m = middle(acc); for (let i = 0; i < N; i++) pipeOut[i] += m[i] / banks;
    });
    const bodyOut = circular(body, t => filter(filter(t, 'lp', spec.bodyF, 0.8), 'hp', 28, 0.7));
    const knockOut = circular(knock, t => filter(t, 'bp', spec.knockBand[0], spec.knockBand[1]));
    const tickOut = circular(tick, t => filter(t, 'hp', 3500, 0.7));
    // intake: noise following the firing envelope, rising with rpm
    const nz = white(N, r), intakeRaw = mk(N);
    for (let i = 0; i < N; i++) intakeRaw[i] = nz[i] * Math.min(1.5, env[i]);
    const intakeOut = circular(intakeRaw, t => filter(filter(t, 'bp', 900 + rpm * 0.25, 0.7), 'lp', 4500, 0.7));
    const rpmFactor = Math.pow(rpm / 3000, 1.2);
    const mix = mk(N);
    const ref = Math.max(1e-9, peak(pipeOut));
    for (let i = 0; i < N; i++) mix[i] = pipeOut[i] / ref;
    const bodyRef = Math.max(1e-9, peak(bodyOut)), knockRef = Math.max(1e-9, peak(knockOut)), intakeRef = Math.max(1e-9, peak(intakeOut)), tickRef = Math.max(1e-9, peak(tickOut));
    for (let i = 0; i < N; i++) {
        mix[i] += bodyOut[i] / bodyRef * spec.body * 0.7 * (0.4 + 0.6 * clamp(rpm / 2500, 0, 1)) + knockOut[i] / knockRef * (spec.knock * 0.8 + 0.22) * (0.35 + 0.65 * clamp(rpm / 2500, 0, 1))
            + intakeOut[i] / intakeRef * spec.intake * 0.5 * (0.9 + rpmFactor) + tickOut[i] / tickRef * spec.tick * 0.12 * clamp(rpm / 2500, 0.3, 1);
    }
    // continuous mechanical bed (chain, gears, valvetrain wash), strongest where single firings would otherwise sound like drum hits
    {
        const wash = circular(white(N, r), t => filter(filter(t, 'bp', 1300, 0.45), 'lp', 3600, 0.7)), washRef = Math.max(1e-9, peak(wash)), idleWeight = clamp(1.3 - rpm / 3000, 0.25, 1.3);
        for (let i = 0; i < N; i++) mix[i] += wash[i] / washRef * (0.32 * idleWeight) * (0.55 + 0.45 * Math.min(1, env[i] / 2));
    }
    if (spec.turbo) {
        const t = spec.turbo, f = Math.round(rpm / 60 * t.ratio * 2) / 2, level = t.gain * clamp((rpm - t.start) / 3500, 0, 1.2) ** 1.4 * 0.5;
        const wob1 = loopHz(N, 1);
        for (let i = 0; i < N; i++) {
            const tt = i / SR;
            mix[i] += level * (0.8 * Math.sin(TAU * f * tt) + 0.18 * Math.sin(TAU * f * 2 * tt)) * (1 + 0.06 * Math.sin(TAU * wob1 * tt));
        }
        const noise = circular(white(N, r), tt => filter(tt, 'bp', 5200, 1.4));
        for (let i = 0; i < N; i++) mix[i] += noise[i] * level * 0.35;
    }
    if (spec.blade) { // blade whirr: broad noise, amplitude-modulated at blade passing frequency
        const bn = circular(white(N, r), tt => filter(tt, 'bp', 1400, 0.6)), bf = loopHz(N, rpm / 60 * 2);
        for (let i = 0; i < N; i++) mix[i] += bn[i] * spec.blade * 0.4 * (0.6 + 0.4 * Math.sin(TAU * bf * i / SR)) * rpmFactor * 0.6;
    }
    const sat = softClip(mix, spec.drive * 0.7);
    const out = circular(sat, t => filter(filter(filter(dcBlock(t), 'hp', 55 + 45 * clamp(1 - rpm / 2500, 0, 1), 0.8), 'lp', spec.lp, 0.7), 'peak', 220, 0.8, 2.5));
    return out;
}

function renderElectric(rpmNominal) {
    const r = rng(seedOf(`engine:couch:${rpmNominal}`));
    const out = mk(N), rpm = Math.round(rpmNominal / 60) * 60, rev = rpm / 60;
    const mesh = Math.round(rev * 19 * 2) / 2, motor = Math.round(rev * 6 * 2) / 2 + 60;
    const am = loopHz(N, Math.max(1, Math.round(rev)));
    const gain = 0.5 + 0.5 * clamp(rpm / 6600, 0, 1);
    for (let i = 0; i < N; i++) {
        const t = i / SR, wob = 1 + 0.0025 * Math.sin(TAU * loopHz(N, 3) * t), amod = 1 + 0.18 * Math.sin(TAU * am * t);
        let v = 0;
        [[1, 1], [2, 0.55], [3, 0.32], [4, 0.16], [5, 0.08]].forEach(([m, a]) => { v += a * Math.sin(TAU * mesh * m * wob * t); });
        v *= amod * 0.7;
        v += 0.55 * Math.sin(TAU * motor * wob * t) * gain + 0.28 * Math.sin(TAU * motor * 2 * t) * gain + 0.12 * Math.sin(TAU * motor * 3.01 * t);
        v += 0.5 * Math.sin(TAU * 100 * t) * 0.3 + 0.2 * Math.sin(TAU * 50 * t);
        out[i] = v;
    }
    // wind rush of the motor fan
    const fan = circular(white(N, r), t => filter(filter(t, 'bp', 1100 + rpm * 0.3, 0.7), 'lp', 5000));
    for (let i = 0; i < N; i++) out[i] += fan[i] * 0.22 * gain;
    // springs and upholstery: squeaks whose count grows with rpm, each a rubbed resonance sweeping upward
    const squeaks = Math.round(2 + rpm / 1400), sq = mk(N);
    for (let s = 0; s < squeaks; s++) {
        const at = Math.round((s + 0.15 + 0.7 * r()) * N / squeaks), len = Math.round((0.07 + 0.09 * r()) * SR);
        const f0 = 750 + r() * 900, up = 1.15 + r() * 0.6, ph = r() * TAU, g = 0.25 + r() * 0.3;
        const seg = mk(len); let phase = ph;
        for (let j = 0; j < len; j++) {
            const x = j / len, f = f0 * (1 + (up - 1) * (x < 0.5 ? x * 2 : 1 - (x - 0.5) * 0.7)) * (1 + 0.04 * Math.sin(TAU * 32 * j / SR));
            phase += TAU * f / SR;
            const rub = Math.sin(phase) + 0.4 * Math.sin(phase * 2.01) + 0.15 * (r() * 2 - 1);
            seg[j] = rub * Math.sin(Math.PI * x) ** 1.5 * g;
        }
        addWrap(sq, seg, at);
    }
    const sqf = circular(sq, t => filter(t, 'bp', 1700, 0.8));
    for (let i = 0; i < N; i++) out[i] += sqf[i] * 0.55;
    return circular(softClip(out, 0.9), t => filter(dcBlock(t), 'lp', 9000, 0.7));
}

export function renderEngineLayer(carId, layer) {
    const x = carId === 'couch' ? renderElectric(ENGINE_LAYERS[layer]) : renderCombustion(carId, ENGINE_LAYERS[layer]);
    return [x];
}
