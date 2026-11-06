// Aura Drift weather contract: seeded schedule, smooth transitions, gusts, lightning and wet roads.
//   node scripts/test-aura-drift-weather.mjs
// The simulation uses only the seeded rng at construction time; every later value is a closed-form
// function of simulation time, so an input tape must reproduce the exact same weather.
import { loadSim } from './aura-drift-sim-loader.mjs';

const AD = loadSim();
const BIOMES = ['alpine', 'desert', 'harbour', 'salt', 'forest', 'industry', 'glacier', 'metro'];
const DT = 1 / 120;
let checks = 0, failures = 0;
const ok = (cond, message) => {
    checks++;
    if (!cond) { failures++; console.error('FAIL: ' + message); }
};
const track = biomeId => ({ biomeId });

function sample(cond) {
    const s = cond.state;
    return [s.time, s.kind, s.prevKind, s.intensity, s.grip, s.wind, s.gust,
        s.wetness, s.flash, s.visibility, s.blend]
        .map(v => typeof v === 'number' ? v.toFixed(9) : v).join('|');
}

// ---- 1. schedule shape and storm placement -------------------------------------------------
for (const biome of BIOMES) {
    const profile = AD.CONDITIONS[biome];
    const s = AD.createConditions(track(biome), 0x51ce + biome.length).state;
    ok(s.schedule.length === 6, `${biome}: six scheduled conditions`);
    ok(s.schedule.every((e, i) => i === 0 || e.at > s.schedule[i - 1].at), `${biome}: schedule is strictly increasing`);
    ok(s.schedule.every(e => profile.kinds.includes(e.kind)), `${biome}: only biome conditions`);
    ok(s.schedule.every(e => e.intensity >= 0.35 && e.intensity <= 1), `${biome}: intensity in range`);
    ok(s.strikes.every((e, i) => i === 0 || e.at >= s.strikes[i - 1].at), `${biome}: strikes are sorted`);
    // A strike must fall inside a rain/snow segment of the schedule.
    ok(s.strikes.every(strike => {
        let owner = -1;
        for (let i = 0; i < s.schedule.length; i++) if (strike.at >= s.schedule[i].at) owner = i;
        return owner >= 0 && (s.schedule[owner].kind === 'rain' || s.schedule[owner].kind === 'snow');
    }), `${biome}: lightning only inside storm segments`);
}
ok(AD.createConditions(track('harbour'), 7717).state.strikes.length > 0, 'harbour: at least one strike');
ok(AD.createConditions(track('glacier'), 7717).state.strikes.length > 0, 'glacier: at least one strike');
ok(AD.createConditions(track('desert'), 7717).state.strikes.length === 0, 'desert: no thunder in the heat');
ok(AD.createConditions(track('salt'), 7717).state.strikes.length === 0, 'salt flats: no thunder in the glare');

// ---- 2. deterministic replay ---------------------------------------------------------------
for (const biome of BIOMES) {
    const a = AD.createConditions(track(biome), 0xa17a9 + biome.length);
    const b = AD.createConditions(track(biome), 0xa17a9 + biome.length);
    const c = AD.createConditions(track(biome), 0xb17a9 + biome.length);
    let identical = true, different = false;
    for (let i = 0; i < 36000; i++) {
        a.update(DT); b.update(DT); c.update(DT);
        if (i % 97 === 0) {
            if (sample(a) !== sample(b)) identical = false;
            if (sample(a) !== sample(c)) different = true;
        }
    }
    ok(identical, `${biome}: identical seed reproduces the same weather`);
    ok(different, `${biome}: another seed changes the weather`);
}

// ---- 3. bounds, transitions and events over a full race -------------------------------------
for (const biome of BIOMES) {
    const cond = AD.createConditions(track(biome), 0xc0ffee + biome.length);
    const s = cond.state;
    let kinds = new Set(), lightning = 0, gusts = 0, lastGust = -100;
    let minGrip = Infinity, maxGrip = -Infinity, maxFlash = 0;
    let sawWet = false, sawBlend = false, wetLingers = false, wetKindEndedAt = -100;
    let boundsOk = true, cuesOk = true;
    for (let i = 0; i < 48000; i++) {
        const prev = s.kind, prevWet = prev === 'rain' || prev === 'snow';
        cond.update(DT);
        const t = s.time;
        kinds.add(s.kind);
        minGrip = Math.min(minGrip, s.grip); maxGrip = Math.max(maxGrip, s.grip);
        maxFlash = Math.max(maxFlash, s.flash);
        if (s.wetness > 0.1) sawWet = true;
        if (s.blend > 0 && s.blend < 1) sawBlend = true;
        if (prevWet && s.kind !== prev) wetKindEndedAt = t;
        if (wetKindEndedAt > 0 && t - wetKindEndedAt > 2 && t - wetKindEndedAt < 8 && s.wetness > 0.1) wetLingers = true;
        boundsOk = boundsOk && Number.isFinite(s.grip) && Number.isFinite(s.wind) && Number.isFinite(s.flash) &&
            s.grip >= 0.6 && s.grip <= 1.0001 && s.intensity >= 0 && s.intensity <= 1 &&
            s.gust >= 0 && s.gust <= 1 && s.wetness >= 0 && s.wetness <= 1 &&
            s.flash >= 0 && s.flash <= 1.1 && s.visibility >= 0.35 && s.visibility <= 1;
        for (const evt of s.events) {
            if (evt.type === 'lightning') {
                lightning++;
                cuesOk = cuesOk && evt.strength > 0 && evt.strength <= 1 && evt.thunder > 0;
            }
            if (evt.type === 'gust') {
                gusts++;
                cuesOk = cuesOk && t - lastGust >= 7;
                lastGust = t;
            }
        }
    }
    ok(boundsOk, `${biome}: all weather values stay finite and inside their bounds`);
    ok(cuesOk, `${biome}: lightning and gust cues carry valid payloads and spacing`);
    ok(kinds.size >= 2, `${biome}: weather changes during a race`);
    ok(sawBlend, `${biome}: kinds crossfade instead of snapping`);
    if (s.strikes.length) ok(maxFlash > 0.2, `${biome}: lightning actually lights the sky`);
    if (biome === 'harbour' || biome === 'metro' || biome === 'glacier' || biome === 'alpine') {
        ok(sawWet && wetLingers, `${biome}: wet roads linger after the precipitation stops`);
        ok(lightning > 0, `${biome}: storm produced lightning events`);
    }
    ok(gusts > 0 && gusts < 60, `${biome}: gust cues are bounded (${gusts})`);
}

console.log(`aura-drift weather: ${checks} checks, ${failures} failures`);
if (failures) process.exit(1);
