// Observatory, "Clockwork night": 92 BPM, D minor. A ticking escapement (hat tick, marimba tock), a music-box melody,
// harpsichord arpeggios and a soft pad under the telescope. Pure data, deterministic.
import { midi, drum as D, melody as M, arp, chord, over, rep, pat, stingerSet } from './notation.js';

// r = bass root, set = arpeggio notes (ascending), p = pad voicing.
const CH = {
    Dm: { r: 'D2', set: ['D4', 'F4', 'A4', 'D5', 'F5', 'A5'], p: ['D3', 'A3', 'D4', 'F4'] },
    Bb: { r: 'Bb1', set: ['D4', 'F4', 'Bb4', 'D5', 'F5', 'Bb5'], p: ['Bb2', 'F3', 'Bb3', 'D4'] },
    F: { r: 'F2', set: ['C4', 'F4', 'A4', 'C5', 'F5', 'A5'], p: ['F3', 'C4', 'F4', 'A4'] },
    C: { r: 'C2', set: ['C4', 'E4', 'G4', 'C5', 'E5', 'G5'], p: ['C3', 'G3', 'C4', 'E4'] },
    Gm: { r: 'G2', set: ['D4', 'G4', 'Bb4', 'D5', 'G5', 'Bb5'], p: ['G3', 'D4', 'G4', 'Bb4'] },
    A: { r: 'A1', set: ['C#4', 'E4', 'A4', 'C#5', 'E5', 'A5'], p: ['A2', 'E3', 'A3', 'C#4'] }
};
const prog = (s) => s.split(' ').map((k) => CH[k]);
const MAIN = prog('Dm Bb F C Dm Bb Gm A');
const STILL = prog('Dm Dm Bb Bb Gm Gm A A');
const TENSE = prog('Dm Dm Gm Gm Dm Bb A A');
const WIZ = prog('Bb C Dm A Bb C Gm A');
const BON = prog('F C Bb C F C Gm A');

// Harpsichord arpeggio orders: indexes into the chord set, '.' rests.
const ORD = {
    a: '0.2.1.3.4.3.2.1.', b: '0.1.2.1.3.2.4.3.', c: '0..2.1.3..4.3.2.', soft: '0...2...1...3...',
    mb: '0123432101234321', mb2: '0213243102132431', rise: '0.1.2.3.4.5.4.3.'
};
// Bass patterns for a root midi note r.
const BASS = {
    a: (r) => [[0, r, 3, 0.9], [6, r, 2, 0.6], [8, r + 7, 3, 0.75], [14, r + 7, 1, 0.55]],
    b: (r) => [[0, r, 2, 0.9], [2, r + 12, 1, 0.55], [4, r, 2, 0.75], [6, r + 7, 1, 0.65], [8, r, 2, 0.85], [10, r + 12, 1, 0.55], [12, r, 2, 0.75], [14, r + 7, 1, 0.65]],
    c: (r) => Array.from({ length: 16 }, (_, i) => [i, i % 4 === 3 ? r + 12 : i % 4 === 2 ? r + 7 : r, 1, i % 4 === 0 ? 0.9 : 0.6]),
    soft: (r) => [[0, r, 8, 0.55], [8, r + 7, 8, 0.45]]
};
// Escapement and drums. tick = hat, tock = marimba (pitched per chord, see section()).
const KIT = {
    attract: { tick: D('x...o...x...o...', { vel: 0.7 }) },
    main: { tick: D('X.......x.......', { vel: 0.85 }), snare: D('........g.......') },
    main2: { tick: D('X.......x.......', { vel: 0.85 }), snare: D('........x.......', { vel: 0.6 }) },
    mode: { tick: D('x...x...x...x...', { vel: 0.75 }), kick: D('X.......X...x...', { vel: 0.7 }) },
    mb: { tick: D('x.x.x.x.x.x.x.x.', { vel: 0.8 }), kick: D('X...X...X...X...'), snare: D('....X.......X...') },
    mb2: { tick: D('x.x.x.x.xoxoxoxo', { vel: 0.8 }), kick: D('X...X...X..xX...'), snare: D('....X.......X.xX') },
    wiz: { tick: D('xoxoxoxoxoxoxoxo', { vel: 0.85 }), kick: D('X..xX..xX..xX.xX'), snare: D('....X..g....X.xX') },
    bonus: { tick: D('x.......o.......', { vel: 0.6 }) },
    hi: { tick: D('x.x.x.x.x.x.x.x.', { vel: 0.75 }), kick: D('X...X...X...X...'), snare: D('....X.......X...') }
};
const TOCK = { all: [[4, 0.6], [12, 0.45]], fast: [[2, 0.5], [4, 0.6], [10, 0.5], [12, 0.6]] };

// Music-box melodies (ping), one string per bar.
const BOX = {
    still: ['D5:4 F5:4 A5:8', 'F5:4 E5:4 D5:8', 'D5:4 F5:4 Bb5:8', 'A5:4 F5:4 D5:8', 'G5:4 Bb5:4 D6:8', 'Bb5:4 A5:4 G5:8', 'A5:4 C#6:4 E6:8', 'D6:4 C#6:4 A5:8'],
    a: ['A5:3 F5:1 D5:2 F5:2 A5:4 .:4', 'Bb5:3 A5:1 F5:2 D5:2 F5:4 .:4', 'A5:3 C6:1 A5:2 F5:2 C6:4 .:4', 'G5:3 E5:1 G5:2 C6:2 E6:2 D6:2 .:4',
        'D6:3 A5:1 F5:2 A5:2 D6:4 .:4', 'D6:3 C6:1 Bb5:2 F5:2 Bb5:4 .:4', 'Bb5:2 D6:2 G6:4 F6:2 D6:2 Bb5:4', 'A5:2 C#6:2 E6:4 D6:2 C#6:2 A5:4'],
    b: ['F6:2 E6:2 D6:4 A5:4 .:4', 'D6:2 C6:2 Bb5:4 F5:4 .:4', 'C6:2 Bb5:2 A5:4 F5:4 .:4', 'E6:2 D6:2 C6:4 G5:4 .:4',
        'F6:2 E6:2 D6:2 E6:2 F6:4 A5:4', 'D6:2 C6:2 Bb5:2 C6:2 D6:4 F6:4', 'G6:4 F6:2 D6:2 Bb5:4 D6:4', 'E6:2 D6:2 C#6:4 A5:4 E5:4'],
    c: ['A5:1 D6:1 F6:2 E6:1 D6:1 A5:2 F5:2 A5:2 D6:4', 'Bb5:1 D6:1 F6:2 D6:1 Bb5:1 F5:2 D5:2 F5:2 Bb5:4', 'A5:1 C6:1 F6:2 C6:1 A5:1 F5:2 A5:2 C6:2 F6:4',
        'G5:1 C6:1 E6:2 C6:1 G5:1 E5:2 G5:2 C6:2 E6:4', 'F6:2 D6:2 A5:2 D6:2 F6:2 A6:2 F6:4', 'D6:2 Bb5:2 F5:2 Bb5:2 D6:2 F6:2 D6:4',
        'G5:1 Bb5:1 D6:2 G6:2 F6:2 D6:2 Bb5:2 D6:4', 'A5:2 C#6:2 E6:2 A6:2 G6:2 E6:2 C#6:2 A5:2'],
    mode: ['D5:2 F5:2 A5:2 F5:2 D5:2 F5:2 A5:2 D6:2', 'D5:2 F5:2 A5:2 F5:2 E5:2 F5:2 A5:2 C6:2', 'G5:2 Bb5:2 D6:2 Bb5:2 G5:2 Bb5:2 D6:2 G6:2',
        'G5:2 Bb5:2 D6:2 Bb5:2 A5:2 Bb5:2 D6:2 F6:2', 'D5:2 F5:2 A5:2 F5:2 D5:2 F5:2 A5:2 D6:2', 'Bb5:2 D6:2 F6:2 D6:2 Bb5:2 D6:2 F6:2 Bb6:2',
        'A5:2 C#6:2 E6:2 C#6:2 A5:2 C#6:2 E6:2 A6:2', 'A5:2 C#6:2 E6:2 G6:2 F6:4 E6:4'],
    bonus: ['A5:4 C6:4 F6:8', 'G5:4 C6:4 E6:8', 'Bb5:4 D6:4 F6:8', 'G5:4 C6:4 E6:8', 'A5:4 C6:4 F6:8', 'E6:4 C6:4 G5:8', 'D6:4 Bb5:4 G5:8', 'C#6:4 E6:4 A6:8']
};
// Flute lines.
const FLUTE = {
    low: ['D5:6 F5:2 A5:8', 'A5:4 G5:4 F5:8', 'G5:6 Bb5:2 D6:8', 'D6:4 C6:4 Bb5:8', 'A5:6 F5:2 D5:8', 'F5:4 D5:4 Bb4:8', 'E5:6 A5:2 C#6:8', 'E6:8 D6:4 C#6:2 A5:2'],
    main: ['D5:6 F5:2 A5:8', 'Bb5:6 A5:2 F5:8', 'C6:6 A5:2 F5:8', 'E5:4 G5:4 C6:8', 'D5:6 F5:2 A5:4 D6:4', 'D6:6 C6:2 Bb5:8', 'Bb5:4 D6:4 G6:8', 'E6:6 D6:2 C#6:4 A5:4'],
    wiz: ['F6:4 D6:4 Bb5:4 D6:4', 'G6:4 E6:4 C6:4 E6:4', 'A6:4 F6:4 D6:4 F6:4', 'E6:4 C#6:4 A5:4 C#6:4', 'D6:4 F6:4 Bb6:4 F6:4', 'E6:4 G6:4 C7:4 G6:4', 'D6:2 Bb5:2 G5:4 Bb5:2 D6:2 G6:4', 'A6:8 E6:4 C#6:4'],
    bonus: ['C6:8 A5:8', 'E6:8 C6:8', 'D6:8 Bb5:8', 'E6:8 G5:8', 'C6:8 F6:8', 'E6:8 C6:8', 'D6:8 Bb5:8', 'A5:16']
};
const rows = (list, vel) => over(list, (s) => M(s, { vel }));

function section(o) {
    const chords = o.chords || MAIN;
    const tracks = {};
    if (o.drone) tracks.drone = [[0, midi('D1'), 64, 0.6], [64, midi('D1'), 64, 0.6]];
    if (o.pad) tracks.pad = over(chords, (c) => chord(c.p, { vel: o.padVel || 0.35 }));
    if (o.bass) tracks.bass = over(chords, (c) => BASS[o.bass](midi(c.r)));
    if (o.harp) tracks.harp = over(chords, (c) => arp(c.set, o.harp, { vel: o.harpVel || 0.6 }));
    if (o.harp2) tracks.harp2 = over(chords, (c) => arp(c.set.map((n) => midi(n) + 12), o.harp2, { vel: 0.4 }));
    if (o.box) tracks.box = rows(BOX[o.box], o.boxVel || 0.7);
    if (o.flute) tracks.flute = rows(FLUTE[o.flute], o.fluteVel || 0.7);
    if (o.tock) tracks.tock = over(chords, (c) => TOCK[o.tock].map(([s, v]) => [s, midi(c.r) + 24, 1, v]));
    const kit = KIT[o.kit];
    for (const k of Object.keys(kit)) tracks[k] = rep(kit[k], 8);
    return pat(8, tracks);
}

const patterns = {
    attractA: section({ chords: STILL, drone: true, pad: true, padVel: 0.3, box: 'still', boxVel: 0.55, kit: 'attract' }),
    attractB: section({ chords: STILL, drone: true, pad: true, padVel: 0.3, box: 'still', boxVel: 0.5, harp: ORD.soft, harpVel: 0.45, flute: 'low', fluteVel: 0.45, kit: 'attract' }),
    mainA: section({ bass: 'a', harp: ORD.a, box: 'a', tock: 'all', kit: 'main' }),
    mainB: section({ bass: 'a', harp: ORD.b, box: 'b', flute: 'main', fluteVel: 0.55, tock: 'all', kit: 'main' }),
    mainC: section({ bass: 'b', harp: ORD.c, harp2: ORD.a, box: 'c', tock: 'fast', kit: 'main2' }),
    modeA: section({ chords: TENSE, drone: true, bass: 'b', harp: ORD.rise, harpVel: 0.55, box: 'mode', boxVel: 0.65, tock: 'all', kit: 'mode' }),
    modeB: section({ chords: TENSE, drone: true, bass: 'b', harp: ORD.rise, harpVel: 0.55, box: 'mode', flute: 'low', fluteVel: 0.65, harp2: ORD.a, tock: 'fast', kit: 'mode' }),
    mbA: section({ chords: TENSE, bass: 'c', harp: ORD.mb, harpVel: 0.65, box: 'mode', boxVel: 0.7, tock: 'all', kit: 'mb' }),
    mbB: section({ chords: WIZ, bass: 'c', harp: ORD.mb, harpVel: 0.65, harp2: ORD.mb2, box: 'c', tock: 'fast', kit: 'mb2' }),
    wizard: section({ chords: WIZ, drone: true, pad: true, bass: 'c', harp: ORD.mb, harpVel: 0.7, harp2: ORD.mb2, box: 'c', boxVel: 0.75, flute: 'wiz', fluteVel: 0.85, tock: 'fast', kit: 'wiz' }),
    bonusA: section({ chords: BON, drone: true, pad: true, bass: 'soft', harp: ORD.rise, harpVel: 0.5, box: 'bonus', boxVel: 0.55, kit: 'bonus' }),
    bonusB: section({ chords: BON, drone: true, pad: true, bass: 'soft', harp: ORD.rise, harpVel: 0.5, box: 'bonus', flute: 'bonus', fluteVel: 0.45, kit: 'bonus' }),
    hiscore: section({ chords: WIZ, pad: true, bass: 'c', harp: ORD.mb, harpVel: 0.65, box: 'c', boxVel: 0.7, flute: 'wiz', fluteVel: 0.8, tock: 'all', kit: 'hi' }),
    fill_main: pat(1, {
        tick: D('x.x.x.x.xxxxxxxx', { vel: 0.8 }), tock: [], kick: D('X.......X.......'), snare: D('........x.x.xxXX'),
        box: M('.:8 D6:1 C6:1 Bb5:1 A5:1 G5:1 F5:1 E5:1 D5:1', { vel: 0.8 })
    }),
    fill_mb: pat(1, {
        tick: D('xxxxxxxxxxxxxxxx', { vel: 0.8 }), tock: [], kick: D('X.X.X.X.X.X.XXXX'), snare: D('....X...x.x.xXXX'),
        box: M('D5:1 F5:1 A5:1 D6:1 F6:1 D6:1 A5:1 F5:1 D5:1 F5:1 A5:1 D6:1 F6:2 A6:2', { vel: 0.85 }), harp: []
    }),
    fill_wiz: pat(1, {
        tick: D('xxxxxxxxxxxxxxxx', { vel: 0.85 }), tock: [], kick: D('X.X.X.XXXXXXXXXX'), snare: D('..oo..xx..XX.XXX'),
        box: M('D6:1 F6:1 A6:1 D7:1 A6:1 F6:1 D6:1 A5:1 D6:2 F6:2 A6:2 D7:2', { vel: 0.9 }), harp: []
    }),
    tr_default: pat(1, { tick: D('X.......', { vel: 0.8 }), box: M('.:6 A5:1 D6:1 F6:8', { vel: 0.7 }), harp: [] }),
    tr_big: pat(1, { kick: D('X.......'), box: M('D5:1 F5:1 A5:1 D6:1 F6:2 .:10', { vel: 0.85 }), bass: M('D2:8', { vel: 0.9 }), harp: [] })
};

const ST = stingerSet({
    jackpot: { box: M('D5:1 F5:1 A5:1 D6:1 F6:1 A6:1 D7:1 .:1 D7:6', { vel: 0.9 }), flute: M('A5:8', { vel: 0.8 }), bass: M('D2:8'), kick: D('X.X') },
    superJackpot: { box: M('D5:1 F5:1 A5:1 D6:1 F6:1 A6:1 D7:1 A6:1 F6:1 A6:1 D7:4', { vel: 0.95 }), flute: M('D6:3 F6:3 A6:10', { vel: 0.85 }), bass: M('D2:4 A2:4 D2:8', { vel: 0.9 }), kick: D('X...X...X...XXXX') },
    lock: { bass: M('D2:2 A2:4', { vel: 0.95 }), box: M('D5:1 A5:3', { vel: 0.8 }), tock: M('D4:1', { vel: 0.8 }) },
    lockLit: { box: M('A5:1 .:1 A5:1 .:1 D6:1 .:1 F6:5', { vel: 0.85 }) },
    multiball: { kick: D('X.X.X.X.X.X.X.X.'), snare: D('....X.......X.xX'), bass: M('D2:2 D2:2 D2:2 D2:2 F2:4 G2:4', { vel: 0.9 }), box: M('D5:1 F5:1 A5:1 D6:1 F6:4 A6:8', { vel: 0.85 }) },
    extraBall: { box: M('F5:2 A5:2 C6:2 F6:2 A6:8', { vel: 0.9 }), flute: M('.:6 C6:2 F6:8', { vel: 0.7 }) },
    replay: { box: M('A5:1 A5:1 D6:6', { vel: 0.9 }), bass: M('D2:3') },
    modeStart: { bass: M('D2:2 D2:2 D2:2 .:2', { vel: 0.95 }), tock: M('D4:1 D4:1 F4:1 A4:3', { vel: 0.85 }), flute: M('D4:4 F4:4 A4:8', { vel: 0.75 }) },
    modeComplete: { flute: M('D5:2 F5:2 A5:2 D6:2 F6:8', { vel: 0.85 }), box: M('D5:1 F5:1 A5:1 D6:1 F6:1 A6:2', { vel: 0.8 }), bass: M('D2:4 A2:4') },
    skillShot: { box: M('D5:1 A5:1 D6:1 F6:5', { vel: 0.9 }) },
    ballSaved: { flute: M('F5:2 E5:2 D5:4', { vel: 0.8 }), bass: M('D2:3') },
    drain: { flute: M('A5:4 G5:4 F5:4 D5:4', { vel: 0.65 }), bass: M('D2:8', { vel: 0.7 }) },
    gameOver: { flute: M('F5:4 D5:4 C5:4 D4:4', { vel: 0.7 }), drone: M('D1:16', { vel: 0.8 }), tock: M('D3:2 .:6 D3:2', { vel: 0.7 }) },
    highScore: { box: M('D5:2 F5:2 A5:2 D6:2 .:1 D6:1 F6:1 A6:1 D7:4', { vel: 0.9 }), flute: M('A6:16', { vel: 0.6 }), kick: D('X...X...X'), snare: D('....x...x') },
    tiltWarning: { flute: M('D4:2 Ab4:2 D4:2 Ab4:2', { vel: 0.8 }), tick: D('xxxxxxxx', { vel: 0.8 }), tock: M('D3:1 .:1 D3:1 .:1 D3:1', { vel: 0.9 }) },
    tilt: { bass: M('D2:4 Ab1:4 D2:6', { vel: 1 }), box: M('D3:1 Ab3:1 D3:1 Ab3:1', { vel: 0.9 }), tock: M('D2:1 D2:1 D2:1 D2:1 D2:4', { vel: 1 }) },
    combo: { box: M('F5:1 A5:1 D6:2', { vel: 0.85 }) },
    bonusTick: { box: M('A5:3', { vel: 0.7 }) },
    bonusTotal: { box: M('D5:2 F5:2 A5:2 D6:6', { vel: 0.85 }), bass: M('D2:6') },
    match: { box: M('A5:2 G5:2 F5:2 E5:2 F5:4', { vel: 0.8 }) },
    knocker: { tock: M('D3:3', { vel: 1 }), kick: D('X'), bass: M('D2:3', { vel: 0.9 }) },
    startGame: { tock: M('D4:1 D4:1 F4:1 A4:1 D5:2 .:2', { vel: 0.85 }), flute: M('D4:2 A4:2 D5:6', { vel: 0.8 }), bass: M('D2:4 D2:4') },
    shootAgain: { box: M('D5:2 A5:2 D6:4', { vel: 0.85 }) },
    wizard: { kick: D('X...X...X...XXXX'), snare: D('....X.......X.xX'), flute: M('D4:4 D4:4 F4:4 A4:4', { vel: 0.85 }), bass: M('D2:4 D2:4 F2:4 G2:4'), drone: M('D1:16'), box: M('D5:1 F5:1 A5:1 D6:1 F6:1 A6:1 .:10', { vel: 0.9 }) },
    surprise: { box: M('D5:1 F5:1 A5:1 D6:1 F6:1 A6:5', { vel: 0.9 }), flute: M('A5:2 D6:2 F6:6', { vel: 0.8 }), bass: M('D2:3 A2:3', { vel: 0.9 }), tock: M('D4:1 .:1 F4:1', { vel: 0.8 }) }
});
Object.assign(patterns, ST.patterns);

export default {
    id: 'observatory',
    bpm: 92,
    swing: 0.04,
    reverb: 1.6,
    instruments: {
        drone: { type: 'subDrone', gain: 0.45 },
        pad: { type: 'pad', gain: 0.38, send: 0.55, cutoff: 1100, attack: 0.9, release: 0.9 },
        box: { type: 'ping', gain: 0.55, send: 0.5 },
        harp: { type: 'pluck', gain: 0.5, send: 0.35 },
        harp2: { type: 'pluck', gain: 0.4, send: 0.45 },
        tick: { type: 'hat', gain: 0.28 },
        tock: { type: 'marimba', gain: 0.45, decay: 0.2 },
        bass: { type: 'sawBass', gain: 0.45 },
        kick: { type: 'kick', gain: 0.6, pitch: 110, low: 48 },
        snare: { type: 'snare', gain: 0.4 },
        flute: { type: 'flute', gain: 0.45, send: 0.5, vibrato: 8 }
    },
    patterns,
    sections: {
        off: [],
        attract: ['attractA', 'attractB'],
        main: ['mainA', 'mainB', 'mainC'],
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
