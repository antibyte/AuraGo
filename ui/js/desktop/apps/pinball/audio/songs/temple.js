// Lost Temple of the Sun: tribal, 100 BPM, E minor pentatonic (E G A B D) with marimba, log drums and flute. Pure data.
import { midi, drum as D, melody as M, arp, chord, over, rep, pat, stingerSet } from './notation.js';

const CH = {
    E: { r: 'E2', set: ['E4', 'G4', 'A4', 'B4', 'D5', 'E5'], p: ['E3', 'B3', 'E4', 'G4'] },
    G: { r: 'G2', set: ['G4', 'A4', 'B4', 'D5', 'E5', 'G5'], p: ['G3', 'D4', 'G4', 'B4'] },
    A: { r: 'A1', set: ['A4', 'B4', 'D5', 'E5', 'G5', 'A5'], p: ['A3', 'E4', 'A4', 'B4'] },
    D: { r: 'D2', set: ['D4', 'E4', 'G4', 'A4', 'B4', 'D5'], p: ['D3', 'A3', 'D4', 'E4'] }
};
const bars8 = (...n) => n.map((k) => CH[k]);
const walk = bars8('E', 'E', 'G', 'G', 'E', 'E', 'D', 'A');
const still = bars8('E', 'E', 'E', 'E', 'G', 'G', 'D', 'D');
const wizCh = bars8('E', 'G', 'A', 'D', 'E', 'G', 'A', 'E');
const tenseCh = bars8('E', 'E', 'D', 'D', 'E', 'E', 'A', 'A');

// Polyrhythm: accents on 3+3+3+3+4 against the 4/4 drums.
const ORD = {
    a: '0..2..1..3..2.4.', b: '0.2.1.3.4.3.2.1.', c: '5..4..3..2..1.0.', d: '0123210101232101',
    rise: '0.1.2.3.4.5.4.3.', mb: '0123432101234321'
};
const LOG = {
    a: (r) => [[0, r, 2, 0.95], [3, r, 1, 0.7], [6, r + 7, 1, 0.75], [8, r, 2, 0.85], [11, r, 1, 0.7], [14, r + 7, 1, 0.7]],
    b: (r) => [[0, r, 2, 0.95], [2, r + 12, 1, 0.6], [3, r, 1, 0.7], [6, r + 7, 1, 0.8], [8, r, 2, 0.9], [10, r + 12, 1, 0.6], [11, r, 1, 0.7], [13, r + 7, 1, 0.75], [14, r + 10, 1, 0.7]],
    c: (r) => Array.from({ length: 8 }, (_, i) => [i * 2, i % 4 === 3 ? r + 7 : r, 1, i % 2 ? 0.65 : 0.9]),
    soft: (r) => [[0, r, 3, 0.6], [8, r + 7, 3, 0.5]]
};
const KIT = {
    attract: { shaker: D('x.o.x.o.x.o.x.o.', { vel: 0.7 }), tom: D('X.......g.......', { note: 40, vel: 0.7 }) },
    tribal: {
        kick: D('X.....X...X.....', { vel: 0.85 }), tom: D('......x.....x...', { note: 52 }), tom2: D('.....o...x.o....', { note: 57, vel: 0.9 }),
        shaker: D('xoxoxoxoxoxoxoxo', { vel: 0.7 }), clap: D('....x.......x...', { vel: 0.5 })
    },
    tribal2: {
        kick: D('X.....X..X....X.', { vel: 0.9 }), tom: D('...x..x.x..x..x.', { note: 52 }), tom2: D('.o....o...x.o.x.', { note: 57 }),
        shaker: D('xoxoxoxoxoxoxoxo', { vel: 0.7 }), clap: D('....x.......x..o', { vel: 0.55 })
    },
    mode: {
        kick: D('X..x..X.X..x..X.'), tom: D('.x..x..x.x..x.x.', { note: 52 }), tom2: D('..x..x..x..x.x.x', { note: 57 }),
        shaker: D('xxoxxoxxoxxoxxox', { vel: 0.7 }), clap: D('....X.......X...', { vel: 0.6 })
    },
    mb: {
        kick: D('X.X.X.X.X.X.X.X.'), tom: M('E3:1 E3:1 .:1 E3:1 G3:1 .:1 E3:1 G3:1 .:1 E3:1 G3:1 A3:1 B3:1 .:1 A3:1 G3:1', { vel: 0.85 }),
        tom2: D('..x..x..x..x..x.', { note: 57 }), shaker: D('xxxxxxxxxxxxxxxx', { vel: 0.65 }), clap: D('....X.......X...')
    },
    wiz: {
        kick: D('X..XX..XX..XX.X.'), tom: M('E3:1 .:1 E3:1 G3:1 .:1 E3:1 G3:1 A3:1 B3:2 A3:1 G3:1 E3:1 G3:1 E3:1', { vel: 0.9 }),
        tom2: D('.x..x..x..x..x.x', { note: 57 }), shaker: D('xxxxxxxxxxxxxxxx', { vel: 0.7 }), clap: D('....X.......X.XX')
    },
    bonus: { shaker: D('x.......o.......', { vel: 0.5 }), tom: D('X.......g.......', { note: 40, vel: 0.5 }) },
    hi: {
        kick: D('X...X...X...X...'), tom: D('..x...x...x..x.x', { note: 52 }), tom2: D('.o.o.o.o.o.o.o.o', { note: 57, vel: 0.7 }),
        shaker: D('xoxoxoxoxoxoxoxo', { vel: 0.7 }), clap: D('....X.......X...')
    }
};
const FLUTE = {
    f1: ['E5:6 G5:2 A5:4 B5:4', 'A5:6 G5:2 E5:8', 'G5:4 A5:4 B5:6 A5:2', 'G5:8 E5:8', 'D5:4 E5:4 G5:8', 'A5:6 B5:2 D6:8', 'B5:4 A5:4 G5:4 E5:4', 'E5:12 .:4'],
    f2: ['B5:3 D6:3 B5:3 A5:3 G5:4', 'A5:3 G5:3 E5:3 G5:3 A5:4', 'B5:3 D6:3 E6:3 D6:3 B5:4', 'A5:3 B5:3 A5:3 G5:3 E5:4',
        'D6:3 B5:3 A5:3 B5:3 D6:4', 'E6:3 D6:3 B5:3 A5:3 G5:4', 'G5:3 A5:3 B5:3 D6:3 E6:4', 'E6:8 B5:4 E5:4'],
    wiz: ['E5:4 B5:4 E6:8', 'D6:4 B5:4 G5:8', 'A5:4 E6:4 A6:8', 'G6:4 E6:4 D6:8', 'E6:2 G6:2 B6:4 A6:4 G6:4', 'E6:4 D6:4 B5:8', 'A5:2 B5:2 D6:4 E6:4 G6:4', 'E6:12 .:4'],
    low: ['E4:8 G4:4 A4:4', 'B4:12 A4:4', 'G4:8 E4:8', 'D4:16', 'E4:8 G4:4 A4:4', 'B4:12 D5:4', 'A4:8 G4:8', 'E4:16']
};
const rows = (list, vel) => over(list, (s) => M(s, { vel }));

function section(o) {
    const chords = o.chords || walk;
    const tracks = {};
    if (o.drone) tracks.drone = over(chords, (c, i) => (i % 4 === 0 ? [[0, midi(c.r) - 12, 64, 0.6]] : []));
    if (o.pad) tracks.pad = over(chords, (c) => chord(c.p, { vel: 0.35 }));
    if (o.log) tracks.log = over(chords, (c) => LOG[o.log](midi(c.r)));
    if (o.marimba) tracks.marimba = over(chords, (c) => arp(c.set, o.marimba, { vel: o.marVel || 0.7 }));
    if (o.marimba2) tracks.marimba2 = over(chords, (c) => arp(c.set.map((n) => midi(n) + 12), o.marimba2, { vel: 0.5 }));
    if (o.flute) tracks.flute = rows(o.flute, o.fluteVel || 0.7);
    const kit = KIT[o.kit];
    for (const k of Object.keys(kit)) tracks[k] = rep(kit[k], 8);
    return pat(8, tracks);
}

const patterns = {
    attractA: section({ chords: still, drone: true, marimba: '0.2.1.3.', marVel: 0.5, kit: 'attract' }),
    attractB: section({ chords: still, drone: true, marimba: '0.2.1.3.', marVel: 0.5, log: 'soft', flute: FLUTE.low, fluteVel: 0.5, kit: 'attract' }),
    mainA: section({ marimba: ORD.a, log: 'a', kit: 'tribal' }),
    mainB: section({ marimba: ORD.a, log: 'a', flute: FLUTE.f1, kit: 'tribal' }),
    mainC: section({ marimba: ORD.b, marimba2: ORD.c, log: 'b', flute: FLUTE.f2, fluteVel: 0.6, kit: 'tribal2' }),
    modeA: section({ chords: tenseCh, marimba: ORD.d, marVel: 0.65, log: 'c', kit: 'mode' }),
    modeB: section({ chords: tenseCh, marimba: ORD.d, marVel: 0.65, log: 'c', flute: FLUTE.low, fluteVel: 0.7, marimba2: ORD.a, kit: 'mode' }),
    mbA: section({ chords: tenseCh, marimba: ORD.mb, marVel: 0.75, log: 'b', flute: FLUTE.f2, fluteVel: 0.7, kit: 'mb' }),
    mbB: section({ chords: wizCh, marimba: ORD.mb, marimba2: ORD.d, marVel: 0.75, log: 'c', flute: FLUTE.f2, fluteVel: 0.8, kit: 'mb' }),
    wizard: section({ chords: wizCh, drone: true, pad: true, marimba: ORD.mb, marVel: 0.8, marimba2: ORD.d, log: 'c', flute: FLUTE.wiz, fluteVel: 0.9, kit: 'wiz' }),
    bonusA: section({ chords: wizCh, drone: true, marimba: ORD.rise, marVel: 0.5, log: 'soft', kit: 'bonus' }),
    bonusB: section({ chords: wizCh, drone: true, marimba: ORD.rise, marVel: 0.55, log: 'soft', flute: FLUTE.f1, fluteVel: 0.45, kit: 'bonus' }),
    hiscore: section({ chords: wizCh, marimba: ORD.mb, marVel: 0.7, log: 'c', flute: FLUTE.f2, fluteVel: 0.85, kit: 'hi' }),
    fill_main: pat(1, { kick: D('X.......X.......'), tom: M('.:8 B3:1 B3:1 A3:1 A3:1 G3:1 G3:1 E3:1 E3:1', { vel: 0.9 }), tom2: D('..x...x...x...xx', { note: 57 }), shaker: [], clap: [] }),
    fill_mb: pat(1, { kick: D('X.X.X.X.X.X.XXXX'), tom: M('E3:1 E3:1 G3:1 G3:1 A3:1 A3:1 B3:1 B3:1 A3:1 A3:1 G3:1 G3:1 E3:2 E2:2'), tom2: D('..x..x..x..x..xx', { note: 57 }), shaker: [], clap: D('ooooxxxxxxxxXXXX') }),
    fill_wiz: pat(1, { kick: D('X.X.X.XXXXXXXXXX'), tom: M('E3:1 G3:1 A3:1 B3:1 B3:1 A3:1 G3:1 E3:1 E3:2 G3:2 A3:2 E2:2'), tom2: D('x.x.x.x.x.x.xxxx', { note: 57 }), shaker: [], clap: D('..oo..xx..XX.XXX') }),
    tr_default: pat(1, { kick: D('X'), tom: M('.:8 B3:1 A3:1 G3:1 E3:1 .:4', { vel: 0.8 }), shaker: D('xxxx', { vel: 0.7 }) }),
    tr_big: pat(1, { kick: D('X.......'), tom: M('E3:1 E3:1 G3:1 A3:1 B3:2 .:10', { vel: 0.9 }), log: M('E2:6', { vel: 0.9 }) })
};

const ST = stingerSet({
    jackpot: { marimba: M('E5:1 G5:1 A5:1 B5:1 D6:1 E6:1 G6:1 A6:1 B6:6', { vel: 0.9 }), flute: M('E6:8', { vel: 0.8 }), log: M('E2:8'), tom: D('X.X.X', { note: 52 }) },
    superJackpot: { marimba: M('E5:1 G5:1 A5:1 B5:1 D6:1 E6:1 G6:1 A6:1 B6:1 A6:1 G6:1 E6:1 B6:4', { vel: 0.95 }), flute: M('E6:3 G6:3 B6:10', { vel: 0.85 }), tom: M('E3:1 E3:1 G3:1 G3:1 A3:1 B3:1 .:10', { vel: 0.9 }), kick: D('X...X...X...XXXX') },
    lock: { log: M('E2:2 B2:4', { vel: 0.95 }), marimba: M('E5:1 B5:3', { vel: 0.8 }) },
    lockLit: { marimba: M('E5:1 .:1 E5:1 .:1 B5:1 .:1 E6:5', { vel: 0.85 }) },
    multiball: { tom: M('E3:1 E3:1 G3:1 G3:1 E3:1 E3:1 G3:1 A3:1 B3:2 .:6', { vel: 0.9 }), flute: M('E5:4 B5:4 E6:8', { vel: 0.8 }), log: M('E2:2 E2:2 E2:2 E2:2 G2:4 A2:4'), kick: D('X.X.X.X.X.X.X.X.') },
    extraBall: { marimba: M('G5:2 A5:2 B5:2 D6:2 E6:8', { vel: 0.9 }), flute: M('.:6 B5:2 E6:8', { vel: 0.7 }) },
    replay: { marimba: M('B5:1 B5:1 E6:6', { vel: 0.9 }), log: M('E2:3') },
    modeStart: { log: M('E2:2 E2:2 E2:2 .:2', { vel: 0.95 }), tom: M('E3:1 E3:1 G3:1 A3:3', { vel: 0.9 }), flute: M('E4:4 G4:4 A4:8', { vel: 0.75 }) },
    modeComplete: { flute: M('E5:2 G5:2 A5:2 B5:2 E6:8', { vel: 0.85 }), marimba: M('E5:1 G5:1 A5:1 B5:1 D6:1 E6:1 G6:2', { vel: 0.8 }), log: M('E2:4 B2:4') },
    skillShot: { marimba: M('E5:1 B5:1 D6:1 E6:5', { vel: 0.9 }) },
    ballSaved: { flute: M('A5:2 G5:2 E5:4', { vel: 0.8 }), log: M('E2:3') },
    drain: { flute: M('B5:4 A5:4 G5:4 E5:4', { vel: 0.65 }), log: M('E2:8', { vel: 0.7 }) },
    gameOver: { flute: M('G5:4 E5:4 D5:4 E4:4', { vel: 0.7 }), drone: M('E1:16', { vel: 0.8 }), tom: M('E2:2 .:6 E2:2', { vel: 0.7 }) },
    highScore: { marimba: M('E5:2 G5:2 A5:2 B5:2 .:1 B5:1 D6:1 E6:1 G6:4', { vel: 0.9 }), flute: M('E6:16', { vel: 0.6 }), kick: D('X...X...X'), tom: D('..x...x', { note: 52 }) },
    tiltWarning: { flute: M('E4:2 Bb4:2 E4:2 Bb4:2', { vel: 0.8 }), shaker: D('xxxxxxxx', { vel: 0.8 }), tom: D('X.X.X', { note: 40 }) },
    tilt: { log: M('E2:4 Bb1:4 E2:6', { vel: 1 }), marimba: M('E3:1 Bb3:1 E3:1 Bb3:1', { vel: 0.9 }), tom: M('E2:1 E2:1 E2:1 E2:1 E2:4', { vel: 1 }) },
    combo: { marimba: M('G5:1 B5:1 E6:2', { vel: 0.85 }) },
    bonusTick: { marimba: M('B5:3', { vel: 0.7 }) },
    bonusTotal: { marimba: M('E5:2 G5:2 B5:2 E6:6', { vel: 0.85 }), log: M('E2:6') },
    match: { marimba: M('B5:2 A5:2 G5:2 E5:2 G5:4', { vel: 0.8 }) },
    knocker: { tom: M('E2:3', { vel: 1 }), kick: D('X'), log: M('E2:3', { vel: 0.9 }) },
    startGame: { tom: M('E3:1 E3:1 G3:1 A3:1 B3:2 .:2', { vel: 0.85 }), flute: M('E4:2 B4:2 E5:6', { vel: 0.8 }), log: M('E2:4 E2:4') },
    shootAgain: { marimba: M('E5:2 B5:2 E6:4', { vel: 0.85 }) },
    wizard: { tom: M('E3:1 E3:1 G3:1 G3:1 A3:1 B3:1 .:10', { vel: 0.95 }), flute: M('E4:4 E4:4 G4:4 B4:4', { vel: 0.85 }), log: M('E2:4 E2:4 G2:4 A2:4'), kick: D('X...X...X...XXXX'), drone: M('E1:16') }
});
Object.assign(patterns, ST.patterns);

export default {
    id: 'temple',
    bpm: 100,
    swing: 0.06,
    reverb: 1.2,
    instruments: {
        drone: { type: 'subDrone', gain: 0.5 },
        pad: { type: 'pad', gain: 0.4, send: 0.5, cutoff: 900, attack: 0.8, release: 0.8 },
        log: { type: 'logDrum', gain: 0.85, decay: 0.32 },
        marimba: { type: 'marimba', gain: 0.65, send: 0.3, decay: 0.45 },
        marimba2: { type: 'marimba', gain: 0.5, send: 0.4, decay: 0.3 },
        flute: { type: 'flute', gain: 0.5, send: 0.5, vibrato: 10 },
        kick: { type: 'kick', gain: 0.7, pitch: 120, low: 50 },
        tom: { type: 'tom', gain: 0.75, send: 0.15, decay: 0.3 },
        tom2: { type: 'tom', gain: 0.65, send: 0.15, decay: 0.22 },
        shaker: { type: 'shaker', gain: 0.4, cutoff: 5600 },
        clap: { type: 'clap', gain: 0.35, send: 0.2 }
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
