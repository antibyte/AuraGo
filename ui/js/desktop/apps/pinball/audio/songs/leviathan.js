// Deep Sea Leviathan: dark underwater sonar, 80 BPM, D dorian. Two bars per chord in the calm sections. Pure data.
import { midi, drum as D, melody as M, arp, chord, over, rep, pat, transpose, stingerSet } from './notation.js';

const CH = {
    Dm: { r: 'D2', d: 'D1', p: ['A2', 'C3', 'E3', 'F3'], t: ['D4', 'F4', 'A4', 'C5'] },
    G: { r: 'G1', d: 'G1', p: ['D3', 'G3', 'B3', 'D4'], t: ['G4', 'B4', 'D5', 'G5'] },
    Em: { r: 'E2', d: 'E1', p: ['B2', 'D3', 'G3', 'E3'], t: ['E4', 'G4', 'B4', 'D5'] },
    C: { r: 'C2', d: 'C1', p: ['B2', 'E3', 'G3', 'C4'], t: ['C4', 'E4', 'G4', 'B4'] },
    F: { r: 'F1', d: 'F1', p: ['A2', 'C3', 'E3', 'F3'], t: ['F4', 'A4', 'C5', 'E5'] },
    Bb: { r: 'Bb1', d: 'Bb1', p: ['D3', 'F3', 'Bb3', 'D4'], t: ['Bb3', 'D4', 'F4', 'Bb4'] },
    A: { r: 'A1', d: 'A1', p: ['E3', 'A3', 'C#4', 'E4'], t: ['A3', 'C#4', 'E4', 'A4'] }
};
const bars8 = (...names) => names.map((n) => CH[n]);
const calm = bars8('Dm', 'Dm', 'G', 'G', 'Em', 'Em', 'C', 'C');
const dark = bars8('Dm', 'Dm', 'Bb', 'Bb', 'Dm', 'Dm', 'A', 'A');
const tense = bars8('Dm', 'Dm', 'Bb', 'Bb', 'C', 'C', 'A', 'A');
const perBar = bars8('Dm', 'G', 'Em', 'C', 'Dm', 'G', 'Em', 'C');
const stairs = bars8('Dm', 'Em', 'F', 'G', 'Dm', 'Em', 'F', 'G');
const fan = bars8('Dm', 'G', 'Em', 'C', 'Dm', 'G', 'C', 'Dm');

const BASS = {
    pulse: (r) => [[0, r, 5, 0.8], [6, r, 2, 0.5], [8, r, 5, 0.7], [14, r + 7, 2, 0.5]],
    eighths: (r) => Array.from({ length: 8 }, (_, i) => [i * 2, i % 4 === 3 ? r + 12 : r, 2, i % 2 ? 0.65 : 0.9]),
    drive: (r) => Array.from({ length: 16 }, (_, i) => [i, i % 4 === 3 ? r + 12 : r, 1, i % 4 === 0 ? 0.95 : 0.7])
};
const KIT = {
    deep: { kick: D('X...............', { vel: 0.55 }), tom: M('.:8 D2:3 .:5', { vel: 0.7 }) },
    heart: { kick: D('X.......o.......', { vel: 0.55 }), shaker: D('....o.......g...', { vel: 0.7 }) },
    ticks: { kick: D('X.......o.......', { vel: 0.5 }), shaker: D('o...o...o...o...', { vel: 0.6 }), hat: D('..............g.', { vel: 0.7 }) },
    mode: { kick: D('X..X..X.X.......', { vel: 0.8 }), tom: D('.....g..g.....X.', { note: 45, vel: 0.9 }), shaker: D('o.o.o.o.o.o.o.o.', { vel: 0.7 }) },
    mb: {
        kick: D('X...X...X...X...'), snare: D('....X.......X...', { vel: 0.9 }), hat: D('xoxoxoxoxoxoxoxo', { vel: 0.8 }),
        shaker: D('gogogogogogogogo'), tom: M('.:12 D3:1 D3:1 A2:1 .:1', { vel: 0.8 })
    },
    wiz: {
        kick: D('X..XX..XX..XX.X.'), snare: D('....X.......X.XX'), hat: D('xxxxxxxxxxxxxxxx', { vel: 0.7 }),
        tom: M('.:6 A2:1 .:1 .:2 D3:1 C3:1 A2:1 .:3', { vel: 0.9 }), shaker: D('ogogogogogogogog', { vel: 0.7 })
    },
    bonus: { shaker: D('o.......o.......', { vel: 0.5 }) },
    hi: { kick: D('X...x...X...x...', { vel: 0.9 }), tom: M('.:8 D3:1 .:1 D3:1 .:5', { vel: 0.8 }), hat: D('..x...x...x...x.', { vel: 0.6 }) }
};

const BELL = {
    calm: ['D5:4 F5:4 A5:8', 'G5:4 F5:4 E5:8', 'B4:4 D5:4 G5:8', 'E5:4 D5:4 B4:8', 'G5:4 B5:4 D6:8', 'B5:4 A5:4 G5:8', 'E5:4 G5:4 B5:8', 'A5:4 G5:4 D5:8'],
    tense: ['D5:2 .:2 F5:2 .:2 A5:2 .:2 G5:2 F5:2', 'D5:2 .:2 F5:2 .:2 Bb5:2 .:2 A5:2 F5:2', 'E5:2 .:2 G5:2 .:2 C6:2 .:2 B5:2 G5:2', 'C#5:2 .:2 E5:2 .:2 A5:2 .:2 G5:2 E5:2'],
    call: ['A4:12 G4:4', 'F4:16', 'D4:12 E4:4', 'G4:16', 'B4:12 A4:4', 'G4:16', 'E4:12 G4:4', 'D4:16'],
    hunt: ['A5:2 D6:2 A5:2 F5:2 A5:2 D6:2 F6:2 D6:2', 'D6:2 Bb5:2 F5:2 Bb5:2 D6:2 Bb5:2 F5:2 D5:2', 'E6:2 C6:2 G5:2 C6:2 E6:2 C6:2 G5:2 E5:2', 'C#6:2 A5:2 E5:2 A5:2 C#6:2 A5:2 E6:2 C#6:2'],
    wiz: ['A5:4 D6:4 F6:4 A6:4', 'F6:4 D6:4 Bb5:8', 'G6:4 E6:4 C6:8', 'E6:6 C#6:2 A5:8', 'A5:4 D6:4 F6:4 A6:4', 'D7:8 Bb6:4 F6:4', 'E7:8 C7:4 G6:4', 'A6:12 E6:4'],
    hi: ['D5:2 F5:2 A5:4 D6:4 A5:4', 'B5:2 G5:2 D5:4 G5:8', 'G5:2 B5:2 D6:4 B5:2 G5:2 E5:4', 'C6:4 B5:4 G5:4 E5:4', 'F5:2 A5:2 D6:4 F6:4 E6:2 D6:2', 'D6:2 B5:2 G5:2 B5:2 D6:8', 'E6:4 D6:4 C6:4 G5:4', 'D6:12 A5:4'],
    bonus: ['D5:2 F5:2 A5:2 C6:2 E6:2 C6:2 A5:2 F5:2']
};
const rows = (list, vel) => over(list, (s) => M(s, { vel }));
const PING = {
    slow: ['A5:1 .:15', '.:16', 'E6:1 .:7 A5:1 .:7', '.:16', 'D6:1 .:15', '.:16', 'A5:1 .:15', '.:16'],
    steady: ['A5:1 .:15', 'D6:1 .:7 A5:1 .:7', 'E6:1 .:15', 'A5:1 .:7 F5:1 .:7', 'D6:1 .:15', 'A5:1 .:7 E5:1 .:7', 'E6:1 .:15', 'A5:1 .:3 D6:1 .:3 A6:1 .:7'],
    fast: ['A5:1 .:3 A5:1 .:3 D6:1 .:3 A5:1 .:3'],
    none: []
};

// One 8 bar pattern. o: chords, bass, kit, ping, bell rows, call rows, extras.
function section(o) {
    const chords = o.chords || calm;
    const tracks = {
        drone: over(chords, (c, i) => (i % 2 === 0 || o.dronePerBar ? [[0, midi(c.d), o.dronePerBar ? 16 : 32, 0.7]] : [])),
        pad: over(chords, (c, i) => (i % 2 === 0 || o.padPerBar ? chord(c.p, { len: o.padPerBar ? 16 : 32, vel: o.padVel || 0.5 }) : []))
    };
    if (o.bass) tracks.bass = over(chords, (c) => BASS[o.bass](midi(c.r)));
    if (o.ping) tracks.ping = o.ping.length === 1 ? rep(M(o.ping[0], { vel: 0.8 }), 8) : rows(o.ping, 0.8);
    if (o.bell) tracks.bell = rows(o.bell, o.bellVel || 0.7);
    if (o.call) tracks.call = rows(o.call, o.callVel || 0.6);
    if (o.arp) tracks.arpb = over(chords, (c) => arp(c.t, o.arp, { vel: 0.5 }));
    const kit = KIT[o.kit];
    for (const k of Object.keys(kit)) tracks[k] = rep(kit[k], 8);
    return pat(8, tracks);
}
const tenseBell = [].concat(BELL.tense, BELL.tense);
const huntBell = [].concat(BELL.hunt, BELL.hunt);

const patterns = {
    attractA: section({ ping: PING.slow, kit: 'heart', bell: BELL.calm, bellVel: 0.45 }),
    attractB: section({ ping: PING.slow, kit: 'heart', bell: BELL.calm, bellVel: 0.5 }),
    // The main bed stays dark: drone, pad, sonar pings with long echoes and a sparse heartbeat on a dark progression. No hats, bells, flute or arp.
    mainA: section({ chords: dark, ping: PING.steady, kit: 'deep' }),
    mainB: section({ chords: dark, ping: PING.steady, kit: 'deep' }),
    mainC: section({ chords: dark, padPerBar: true, dronePerBar: true, ping: PING.steady, kit: 'deep' }),
    mainD: section({ chords: dark, bass: 'pulse', ping: PING.steady, kit: 'deep' }),
    modeA: section({ chords: tense, bass: 'eighths', ping: PING.steady, kit: 'mode' }),
    modeB: section({ chords: tense, bass: 'eighths', ping: PING.fast, kit: 'mode', bell: tenseBell, call: BELL.call }),
    mbA: section({ chords: tense, bass: 'drive', ping: PING.fast, kit: 'mb', bell: huntBell, bellVel: 0.7, arp: '0123210301232103' }),
    mbB: section({ chords: tense, bass: 'drive', ping: PING.fast, kit: 'mb', bell: huntBell, bellVel: 0.8, call: BELL.call, callVel: 0.7, arp: '0123210301232103' }),
    wizard: section({ chords: tense, bass: 'drive', ping: PING.fast, kit: 'wiz', bell: BELL.wiz, bellVel: 0.85, call: transposeRows(BELL.call, 12), callVel: 0.7, padVel: 0.65, arp: '0123012301230123' }),
    bonusA: section({ chords: stairs, padPerBar: true, dronePerBar: true, kit: 'bonus', ping: PING.slow, bell: [].concat(BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus), bellVel: 0.55 }),
    bonusB: section({ chords: stairs, padPerBar: true, dronePerBar: true, kit: 'bonus', ping: PING.steady, arp: '0.1.2.3.', bell: [].concat(BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus, BELL.bonus), bellVel: 0.65 }),
    hiscore: section({ chords: fan, padPerBar: true, dronePerBar: true, bass: 'pulse', kit: 'hi', bell: BELL.hi, bellVel: 0.85, call: BELL.call, callVel: 0.6, ping: PING.steady }),
    fill_main: pat(1, { kick: D('X.......X.......', { vel: 0.7 }), tom: M('.:8 D3:2 D3:2 A2:2 D2:2', { vel: 0.8 }), shaker: D('o.o.o.o.o.o.oooo', { vel: 0.7 }), hat: [] }),
    fill_mb: pat(1, { kick: D('X.X.X.X.X.X.XXXX'), snare: D('ooooxxxxxxxxXXXX'), tom: M('.:4 A3:1 A3:1 F3:1 F3:1 D3:1 D3:1 A2:1 A2:1 .:4'), hat: [], shaker: [] }),
    fill_wiz: pat(1, { kick: D('X.X.X.XXXXXXXXXX'), snare: D('..oo..xx..XX.XXX'), tom: M('.:2 F3:1 F3:1 .:2 D3:1 D3:1 .:2 A2:1 A2:1 .:2 D2:2'), hat: [], shaker: [] }),
    tr_default: pat(1, { kick: D('X', { vel: 0.8 }), tom: M('.:8 F3:2 D3:2 A2:2 D2:2', { vel: 0.7 }), ping: M('A5:1', { vel: 0.8 }) }),
    tr_big: pat(1, { kick: D('X.......'), tom: M('D2:2 A2:2 D3:2 A3:2 .:8', { vel: 0.9 }), ping: M('D6:1', { vel: 0.9 }) })
};

function transposeRows(list, n) {
    // Re-spell note names an octave up by shifting the digit; keeps the rows readable in one place.
    return list.map((s) => s.replace(/([A-G][#b]?)(\d)/g, (m, name, oct) => name + (Number(oct) + n / 12)));
}

const ST = stingerSet({
    jackpot: { bell: M('D5:1 F5:1 A5:1 D6:1 F6:1 A6:1 D7:6', { vel: 0.85 }), ping: M('A5:8', { vel: 0.8 }), pad: chord(['A2', 'C3', 'E3', 'F3'], { step: 6, len: 10 }), drone: M('D1:14') },
    superJackpot: { bell: M('D5:1 F5:1 A5:1 D6:1 F6:1 A6:1 D7:1 A6:1 F6:1 D6:1 A5:1 D6:1 F6:1 A6:1 D7:2', { vel: 0.9 }), ping: M('D6:1 .:7 A6:1', { vel: 0.9 }), pad: chord(['A2', 'C3', 'E3', 'F3', 'A3'], { len: 16, vel: 0.6 }), tom: M('D2:1 D2:1 A2:1 A2:1 D3:1 .:3'), kick: D('X...X...X') },
    lock: { ping: M('A5:1 .:3 D6:2', { vel: 0.85 }), bell: M('D5:2 A4:4', { vel: 0.7 }), kick: D('X', { vel: 0.8 }) },
    lockLit: { ping: M('D6:1 .:3 D6:1 .:3 A6:1 .:3', { vel: 0.85 }) },
    multiball: { drone: M('D1:14'), tom: M('D2:1 D2:1 A2:1 A2:1 D3:1 D3:1 A3:1 A3:1 .:8', { vel: 0.9 }), call: M('A4:4 D5:4 F5:8', { vel: 0.8 }), kick: D('X.X.X.X.XXXX') },
    extraBall: { bell: M('D5:2 F5:2 A5:2 C6:2 E6:8', { vel: 0.85 }), ping: M('.:8 A6:1', { vel: 0.7 }) },
    replay: { bell: M('A5:1 A5:1 D6:6', { vel: 0.85 }), ping: M('A5:1', { vel: 0.7 }) },
    modeStart: { drone: M('D1:12'), ping: M('A5:1', { vel: 0.9 }), call: M('D4:4 F4:4 A4:8', { vel: 0.7 }), tom: M('D2:1 D2:1 A2:1', { vel: 0.9 }) },
    modeComplete: { bell: M('D5:2 F5:2 A5:2 D6:2 F6:2 A6:6', { vel: 0.85 }), pad: chord(['A2', 'C3', 'E3', 'F3'], { step: 4, len: 12 }), kick: D('X') },
    skillShot: { ping: M('D5:1 A5:1 D6:1 F6:5', { vel: 0.9 }) },
    ballSaved: { bell: M('A5:2 F5:2 D5:4', { vel: 0.8 }), pad: chord(['D3', 'F3', 'A3'], { step: 4, len: 4, vel: 0.45 }) },
    drain: { call: M('F5:4 E5:4 D5:4 A4:4', { vel: 0.7 }), drone: M('D1:16'), tom: M('D2:6', { vel: 0.7 }) },
    gameOver: { call: M('A4:4 F4:4 E4:4 D4:4', { vel: 0.7 }), drone: M('D1:16', { vel: 0.8 }), pad: chord(['D3', 'F3', 'A3'], { len: 16, vel: 0.4 }) },
    highScore: { bell: M('D5:2 F5:2 A5:2 D6:2 .:1 A5:1 D6:1 F6:1 A6:4', { vel: 0.9 }), kick: D('X...X'), pad: chord(['D3', 'A3', 'D4', 'F4'], { step: 8, len: 8 }) },
    tiltWarning: { ping: M('Ab4:2 D4:2 Ab4:2 D4:2', { vel: 0.9 }), drone: M('D1:8') },
    tilt: { drone: M('D1:12', { vel: 1 }), ping: M('Ab4:1 D4:1 Ab4:1 D4:1 Ab3:4', { vel: 0.9 }), tom: M('D2:1 .:1 D2:1 .:1 D2:2', { vel: 1 }), kick: D('X.X.X') },
    combo: { ping: M('D6:1 A6:3', { vel: 0.85 }) },
    bonusTick: { bell: M('A5:3', { vel: 0.7 }) },
    bonusTotal: { bell: M('D5:2 F5:2 A5:2 D6:6', { vel: 0.8 }), pad: chord(['D3', 'A3', 'D4'], { step: 6, len: 6 }) },
    match: { bell: M('A5:2 F5:2 D5:2 A4:2 D5:4', { vel: 0.75 }) },
    knocker: { tom: M('D2:3', { vel: 1 }), kick: D('X') },
    startGame: { ping: M('A5:1', { vel: 0.9 }), call: M('D4:2 A4:2 D5:6', { vel: 0.75 }), drone: M('D1:12') },
    shootAgain: { ping: M('D6:1 .:3 D6:1 .:2', { vel: 0.85 }), bell: M('A5:2 D6:4', { vel: 0.7 }) },
    wizard: { drone: M('D1:16'), call: M('D4:4 D4:4 F4:4 A4:4', { vel: 0.85 }), tom: M('D2:1 D2:1 A2:1 A2:1 D3:1 .:3', { vel: 0.9 }), kick: D('X...X...X...XXXX'), bell: M('.:8 A5:2 D6:2 F6:4') }
});
Object.assign(patterns, ST.patterns);

export default {
    id: 'leviathan',
    bpm: 80,
    swing: 0,
    reverb: 2.2,
    // Level-matched to the other tables (main ~ -22 LUFS); dark: low cutoffs, deep sub drone, long sonar echo, quiet arp.
    instruments: {
        drone: { type: 'subDrone', gain: 1.0 },
        pad: { type: 'pad', gain: 0.8, send: 0.85, cutoff: 600, attack: 1.6, release: 1.8 },
        bass: { type: 'sawBass', gain: 0.85, cutoff: 480, sub: 1.0 },
        ping: { type: 'ping', gain: 1.0, send: 0.7, echo: 0.5625, feedback: 0.64 },
        bell: { type: 'pluck', gain: 0.6, send: 0.8, bright: 2200, ring: 0.25 },
        call: { type: 'flute', gain: 0.5, send: 0.95, vibrato: 16 },
        arpb: { type: 'squareArp', gain: 0.1, send: 0.8, cutoff: 700 },
        kick: { type: 'kick', gain: 0.9, pitch: 100, low: 38 },
        snare: { type: 'snare', gain: 0.45, send: 0.5, tone: 140 },
        hat: { type: 'hat', gain: 0.1, cutoff: 6500 },
        tom: { type: 'tom', gain: 0.9, send: 0.5, decay: 0.6 },
        shaker: { type: 'shaker', gain: 0.18, cutoff: 3200 }
    },
    patterns,
    sections: {
        off: [],
        attract: ['attractA', 'attractB'],
        main: ['mainA', 'mainB', 'mainC', 'mainD'],
        mode: ['modeA', 'modeB'],
        multiball: ['mbA', 'mbB'],
        wizard: ['wizard'],
        bonus: ['bonusA', 'bonusB'],
        hiscore: ['hiscore']
    },
    fills: { main: 'fill_main', mode: 'fill_main', multiball: 'fill_mb', wizard: 'fill_wiz' },
    transitions: { default: 'tr_default', wizard: 'tr_big', multiball: 'tr_big' },
    stingers: ST.map
};
