// Cyber Neon Metropolis: synthwave, 112 BPM, A minor (Am-F-C-G). Pure data, DOM-free.
import { midi, drum as D, melody as M, arp, chord, over, rep, merge, pat, transpose, stingerSet } from './notation.js';

const CH = {
    Am: { r: 'A1', t: ['A3', 'C4', 'E4', 'A4'], p: ['E3', 'A3', 'C4', 'E4'] },
    F: { r: 'F1', t: ['F3', 'A3', 'C4', 'F4'], p: ['F3', 'A3', 'C4', 'F4'] },
    C: { r: 'C2', t: ['C4', 'E4', 'G4', 'C5'], p: ['G3', 'C4', 'E4', 'G4'] },
    G: { r: 'G1', t: ['G3', 'B3', 'D4', 'G4'], p: ['G3', 'B3', 'D4', 'G4'] },
    E: { r: 'E1', t: ['E3', 'G#3', 'B3', 'E4'], p: ['E3', 'G#3', 'B3', 'D4'] }
};
const prog = (...names) => names.map((n) => CH[n]);
const BASS = {
    roll: (r) => [[0, r, 1, 0.9], [2, r, 1, 0.7], [4, r + 12, 1, 0.8], [6, r, 1, 0.7], [8, r, 1, 0.9], [10, r, 1, 0.7], [12, r + 12, 1, 0.8], [14, r + 7, 1, 0.75]],
    sync: (r) => [[0, r, 3, 0.95], [3, r, 2, 0.75], [6, r, 2, 0.8], [8, r, 3, 0.9], [11, r + 12, 1, 0.7], [12, r, 2, 0.8], [14, r + 12, 1, 0.75]],
    push: (r) => Array.from({ length: 16 }, (_, i) => [i, i % 4 === 3 ? r + 12 : r, 1, i % 4 === 0 ? 0.95 : 0.7]),
    half: (r) => [[0, r, 7, 0.9], [8, r, 6, 0.8]],
    whole: (r) => [[0, r, 16, 0.8]]
};
const UP = '0123012301230123', ZIG = '0121232101212321', SLOW = '0.2.1.3.0.2.1.3.';

// Drum kits, one bar each.
const KIT = {
    attract: { kick: D('X.......X.......', { vel: 0.8 }), clap: D('........x.......', { vel: 0.7 }), hat: D('o...o...o...o...') },
    drive: { kick: D('X...X...X...X...'), snare: D('....X.......X...', { vel: 0.8 }), clap: D('....x.......x...', { vel: 0.7 }), hat: D('o.x.o.x.o.x.o.x.') },
    drive16: { kick: D('X...X...X...X...'), snare: D('....X.......X..g', { vel: 0.8 }), clap: D('....x.......x...', { vel: 0.7 }), hat: D('xoxoxoxoxoxoxoxo', { vel: 0.8 }) },
    mode: { kick: D('X..x..X...X.X...'), snare: D('....X.......X..g', { vel: 0.85 }), hat: D('xoxoxoxoxoxoxoxo', { vel: 0.7 }) },
    mb: {
        kick: D('X.xxX..xX.xxX.x.'), snare: D('....X..g....X..X', { vel: 0.9 }), clap: D('....x.......x...'),
        hat: D('xoxoxoxoxoxoxoxo'), shaker: D('gogogogogogogogo'), tom: M('.:10 A2:1 .:1 G2:1 .:1 E2:1 .:1', { vel: 0.8 })
    },
    wiz: {
        kick: D('X...X...X..XX.X.'), snare: D('....X.......X.XX'), clap: D('....X.......X...', { vel: 0.8 }),
        hat: D('xxxxxxxxxxxxxxxx', { vel: 0.7 }), shaker: D('ogogogogogogogog', { vel: 0.8 }), tom: M('.:6 A2:1 .:1 .:2 E3:1 D3:1 A2:1 .:3', { vel: 0.9 })
    },
    bonus: { kick: D('X.......o.......', { vel: 0.5 }), shaker: D('o.o.o.o.o.o.o.o.', { vel: 0.8 }) },
    hi: { kick: D('X...X...X...X...'), clap: D('....x.......x...'), hat: D('..x...x...x...x.') }
};

// Lead melodies, one string per bar (each sums to 16 steps).
const LEAD = {
    a: ['A4:2 C5:2 E5:4 D5:2 C5:2 .:2 A4:2', 'F5:3 E5:1 C5:4 A4:2 C5:2 .:4', 'G4:2 C5:2 E5:4 G5:4 E5:2 D5:2', 'D5:4 B4:2 D5:2 G5:6 .:2'],
    b: ['E5:2 E5:2 A5:4 G5:2 E5:2 D5:2 E5:2', 'C5:2 F5:2 A5:4 G5:2 F5:2 E5:4', 'G5:4 E5:2 G5:2 C6:4 B5:2 G5:2', 'D5:2 G5:2 B5:4 A5:2 G5:2 .:2 D5:2'],
    c: ['E5:3 D5:1 C5:2 A4:2 C5:4 E5:4', 'A4:2 C5:2 F5:4 E5:2 C5:2 A4:4', 'E5:2 G5:2 E5:2 C5:2 D5:4 E5:4', 'B4:2 D5:2 G5:4 E5:2 D5:2 .:2 B4:2'],
    mode: ['A4:3 .:1 A4:2 C5:2 E5:4 .:4', 'G4:3 .:1 G4:2 B4:2 D5:4 .:4', 'F4:3 .:1 F4:2 A4:2 C5:4 .:4', 'G#4:3 .:1 B4:2 D5:2 E5:4 D5:2 B4:2'],
    wiz: ['C5:4 F5:4 A5:4 C6:4', 'B5:4 G5:4 D5:4 G5:4', 'A5:6 G5:2 E5:4 C5:4', 'E5:2 A5:2 C6:4 B5:2 A5:2 E5:4',
        'F5:2 A5:2 C6:4 A5:2 F5:2 C6:4', 'D6:4 B5:4 G5:4 B5:4', 'G#5:4 B5:4 E6:6 D6:2', 'E6:12 .:4'],
    bonus: ['C5:2 D5:2 E5:2 G5:2 E5:2 D5:2 C5:4', 'A4:2 B4:2 C5:2 E5:2 C5:2 B4:2 A4:4', 'A4:2 C5:2 F5:2 A5:2 F5:2 C5:2 A4:4', 'B4:2 D5:2 G5:2 B5:2 G5:2 D5:2 B4:4'],
    hi1: ['C5:2 E5:2 G5:2 C6:4 G5:2 E5:4', 'B5:2 D6:2 G5:2 B5:4 D6:2 G5:4', 'A5:2 C6:2 E6:4 C6:2 A5:2 E5:4', 'F5:2 A5:2 C6:4 A5:2 F5:2 A5:4'],
    hi2: ['F5:4 A5:4 C6:4 F6:4', 'G5:4 B5:4 D6:4 G6:4', 'C6:2 E6:2 G6:4 E6:2 C6:2 G5:4', 'C6:8 G5:4 E5:4']
};
const melo = (rows, vel) => over(rows, (s) => M(s, { vel }));

// One pattern from a chord progression. o: bass kind, arp order, lead rows, extra tracks.
function section(chords, o) {
    const bars = chords.length;
    const tracks = { bass: over(chords, (c) => BASS[o.bass || 'roll'](midi(c.r))) };
    if (o.arp !== false) tracks.arp = over(chords, (c) => arp(c.t, o.arp || UP, { vel: o.arpVel || 0.55 }));
    tracks.pad = over(chords, (c) => chord(c.p, { vel: o.padVel || 0.5 }));
    if (o.lead) tracks.lead = melo(o.lead, o.leadVel || 0.85);
    if (o.pluck) tracks.pluck = o.pluck;
    const kit = KIT[o.kit];
    for (const k of Object.keys(kit)) tracks[k] = rep(kit[k], bars);
    return pat(bars, tracks);
}

const main = prog('Am', 'F', 'C', 'G');
const modeP = prog('Am', 'G', 'F', 'E');
const wizP = prog('F', 'G', 'Am', 'Am', 'F', 'G', 'E', 'E');
const bonusP = prog('C', 'Am', 'F', 'G');
const hiP1 = prog('C', 'G', 'Am', 'F'), hiP2 = prog('F', 'G', 'C', 'C');
const leadB = melo(LEAD.b, 0.8);

const patterns = {
    attractA: section(main, { bass: 'half', arp: SLOW, arpVel: 0.5, padVel: 0.55, kit: 'attract' }),
    attractB: section(main, { bass: 'half', arp: SLOW, arpVel: 0.5, lead: LEAD.a, leadVel: 0.55, kit: 'attract' }),
    mainA: section(main, { kit: 'drive' }),
    mainB: section(main, { lead: LEAD.a, kit: 'drive' }),
    mainC: section(main, { bass: 'sync', arp: ZIG, arpVel: 0.5, lead: LEAD.b, kit: 'drive16' }),
    mainD: section(main, { lead: LEAD.c, kit: 'drive16', pluck: over(main, (c) => arp(c.p.map((n) => midi(n) + 12), '0.1.2.3.', { vel: 0.5 })) }),
    modeA: section(modeP, { bass: 'sync', arp: ZIG, kit: 'mode' }),
    modeB: section(modeP, { bass: 'sync', arp: ZIG, lead: LEAD.mode, kit: 'mode' }),
    mbA: section(main, { bass: 'push', arp: UP, arpVel: 0.6, lead: LEAD.b, kit: 'mb' }),
    mbB: section(main, { bass: 'push', arp: UP, arpVel: 0.6, lead: LEAD.c, kit: 'mb', pluck: transpose(leadB, 12) }),
    wizard: section(wizP, { bass: 'push', arp: UP, arpVel: 0.65, padVel: 0.6, lead: LEAD.wiz, leadVel: 0.95, kit: 'wiz',
        pluck: transpose(melo(LEAD.wiz, 0.6), -12) }),
    bonusA: section(bonusP, { bass: 'whole', arp: '0.1.2.3.', arpVel: 0.45, padVel: 0.45, kit: 'bonus', lead: LEAD.bonus, leadVel: 0.6 }),
    bonusB: section(bonusP, { bass: 'whole', arp: '0.1.2.3.', arpVel: 0.45, padVel: 0.45, kit: 'bonus', lead: LEAD.bonus, leadVel: 0.7 }),
    hiscoreA: section(hiP1, { bass: 'roll', arp: UP, lead: LEAD.hi1, leadVel: 0.9, kit: 'hi' }),
    hiscoreB: section(hiP2, { bass: 'roll', arp: UP, lead: LEAD.hi2, leadVel: 0.95, kit: 'hi' }),
    // Fills replace the groove tracks of a section's last bar.
    fill_main: pat(1, { kick: D('X.......X.......'), snare: D('..g.o.o.o.xxxxXX'), hat: [], clap: [], tom: M('.:8 E3:1 E3:1 D3:1 D3:1 A2:1 A2:1 G2:1 G2:1', { vel: 0.85 }) }),
    fill_mb: pat(1, { kick: D('X.X.X.X.X.X.XXXX'), snare: D('ooooxxxxxxxxXXXX'), hat: [], clap: [], tom: M('.:4 A3:1 A3:1 E3:1 E3:1 D3:1 D3:1 A2:1 A2:1 .:4') }),
    fill_wiz: pat(1, { kick: D('X.X.X.XXXXXXXXXX'), snare: D('..oo..xx..XX.XXX'), hat: [], clap: [], tom: M('.:2 E3:1 E3:1 .:2 D3:1 D3:1 .:2 A2:1 A2:1 .:2 E2:2') }),
    tr_default: pat(1, { kick: D('X'), clap: D('X', { vel: 0.8 }), hat: D('X.', { vel: 0.9 }).map((e) => [e[0], e[1], 4, e[3]]), tom: M('.:10 A3:1 E3:1 D3:1 A2:1 .:2', { vel: 0.7 }) }),
    tr_wizard: pat(1, { kick: D('X.......'), clap: D('X'), tom: M('A2:1 E3:1 A3:1 E4:1 .:12', { vel: 0.9 }) })
};

const ST = stingerSet({
    jackpot: { lead: M('A4:1 C5:1 E5:1 A5:1 C6:1 E6:1 A5:1 C6:1 E6:6', { vel: 0.9 }), pad: chord(['A3', 'C4', 'E4', 'A4'], { step: 8, len: 7 }), bass: M('A1:8', { vel: 0.9 }), kick: D('X'), clap: D('........X') },
    superJackpot: { lead: M('A4:1 C5:1 E5:1 A5:1 C6:1 E6:1 A6:1 E6:1 A5:1 C6:1 E6:1 A6:1 C7:4', { vel: 0.95 }), pad: chord(['A3', 'C4', 'E4', 'A4', 'E5'], { len: 16, vel: 0.6 }), bass: M('A1:16'), kick: D('X...X...X...XXXX'), snare: D('....ooooxxxxXXXX') },
    lock: { lead: M('E4:2 A4:2 C5:4', { vel: 0.8 }), bass: M('A1:6'), kick: D('X') },
    lockLit: { pluck: M('C5:1 .:1 C5:1 .:1 E5:1 .:1 G5:5', { vel: 0.8 }), bass: M('A1:4', { vel: 0.7 }) },
    multiball: { bass: M('A1:2 A1:2 A1:2 A1:2 F1:4 G1:4'), lead: M('A5:2 A5:1 C6:1 E6:4 D6:2 E6:4', { vel: 0.9 }), kick: D('X.X.X.X.X.X.X.X.'), snare: D('........ooxxXXXX'), tom: M('.:8 E3:1 D3:1 A2:1 E2:1 .:4') },
    extraBall: { lead: M('C5:2 E5:2 G5:2 C6:2 E6:2 G6:6', { vel: 0.9 }), pad: chord(['C4', 'E4', 'G4', 'C5'], { step: 8, len: 8 }), bass: M('C2:8') },
    replay: { lead: M('G5:1 G5:1 G5:2 C6:2 E6:6', { vel: 0.9 }), bass: M('C2:6'), clap: D('X.') },
    modeStart: { bass: M('A1:2 A1:2 A1:2 .:2 C2:4'), lead: M('A4:1 A4:1 C5:1 E5:3 .:2 E5:1 G5:1 A5:6', { vel: 0.85 }), tom: M('A2:1 A2:1 E2:1 .:5', { vel: 0.9 }), kick: D('X.X.X') },
    modeComplete: { lead: M('E5:2 G5:2 A5:2 C6:2 E6:2 A5:2 C6:4', { vel: 0.9 }), pad: chord(['A3', 'C4', 'E4', 'A4'], { step: 8, len: 8 }), kick: D('X.......X'), clap: D('........X') },
    skillShot: { lead: M('E5:1 A5:1 C6:1 E6:1 A6:4', { vel: 0.9 }), bass: M('A1:4', { vel: 0.8 }) },
    ballSaved: { lead: M('E5:2 G5:2 C6:4', { vel: 0.85 }), pad: chord(['C4', 'E4', 'G4'], { step: 4, len: 4 }), bass: M('C2:4') },
    drain: { lead: M('C5:3 B4:3 A4:3 E4:7', { vel: 0.7 }), pad: chord(['A2', 'C3', 'E3'], { len: 16, vel: 0.5 }), bass: M('A1:12', { vel: 0.7 }) },
    gameOver: { lead: M('A4:4 F4:4 E4:4 A3:4', { vel: 0.75 }), pad: chord(['A2', 'F3', 'A3', 'C4'], { len: 16, vel: 0.45 }), bass: M('A1:16', { vel: 0.7 }) },
    highScore: { lead: M('C5:2 E5:2 G5:2 C6:2 .:1 G5:1 C6:1 E6:1 G6:4', { vel: 0.9 }), bass: M('C2:8'), kick: D('X...X...'), clap: D('....X...X') },
    tiltWarning: { lead: M('A3:2 Bb3:2 A3:2 Bb3:2', { vel: 0.8 }), snare: D('X.X.X.X.', { vel: 0.7 }) },
    tilt: { bass: M('A2:2 Eb3:2 A2:2 Eb3:2 A1:8'), lead: M('A3:2 Eb4:2 A3:2 Eb4:2', { vel: 0.8 }), kick: D('X...X...X'), snare: D('..X...X...X') },
    combo: { lead: M('C6:1 E6:1 G6:2', { vel: 0.85 }) },
    bonusTick: { pluck: M('E6:3', { vel: 0.75 }) },
    bonusTotal: { lead: M('C5:2 E5:2 G5:2 C6:6', { vel: 0.85 }), clap: D('......X'), bass: M('C2:8', { vel: 0.8 }) },
    match: { lead: M('G5:2 E5:2 C5:2 G4:2 C5:4', { vel: 0.8 }), pad: chord(['C4', 'E4', 'G4'], { step: 6, len: 6, vel: 0.45 }) },
    knocker: { tom: M('A2:3', { vel: 1 }), kick: D('X'), clap: D('..X', { vel: 0.7 }) },
    startGame: { lead: M('A4:1 C5:1 E5:1 A5:2 .:1 E5:1 A5:1 C6:1 E6:6', { vel: 0.9 }), bass: M('A1:4 A1:4'), kick: D('X...X...') },
    shootAgain: { lead: M('E5:2 A5:2 E6:4', { vel: 0.85 }), bass: M('A1:4', { vel: 0.8 }) },
    wizard: { lead: M('A4:2 A4:2 C5:2 E5:2 A5:4 G5:2 E5:2', { vel: 0.95 }), pad: chord(['F3', 'A3', 'C4', 'F4'], { len: 16, vel: 0.6 }), tom: M('A2:1 A2:1 E3:1 E3:1 A2:1 .:3', { vel: 0.9 }), kick: D('X...X...X...XXXX') }
});
Object.assign(patterns, ST.patterns);

export default {
    id: 'neon',
    bpm: 112,
    swing: 0,
    reverb: 1.2,
    instruments: {
        bass: { type: 'sawBass', gain: 0.7, cutoff: 1500 },
        arp: { type: 'squareArp', gain: 0.45, send: 0.3, cutoff: 2600 },
        pad: { type: 'pad', gain: 0.5, send: 0.6, cutoff: 1100 },
        lead: { type: 'squareArp', gain: 0.5, send: 0.35, cutoff: 4200, sustain: true },
        pluck: { type: 'pluck', gain: 0.55, send: 0.3 },
        kick: { type: 'kick', gain: 0.9 },
        snare: { type: 'snare', gain: 0.55, send: 0.25 },
        hat: { type: 'hat', gain: 0.3 },
        clap: { type: 'clap', gain: 0.45, send: 0.3 },
        tom: { type: 'tom', gain: 0.6, send: 0.15 },
        shaker: { type: 'shaker', gain: 0.4 }
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
        hiscore: ['hiscoreA', 'hiscoreB']
    },
    fills: { main: 'fill_main', mode: 'fill_main', multiball: 'fill_mb', wizard: 'fill_wiz' },
    transitions: { default: 'tr_default', wizard: 'tr_wizard', multiball: 'tr_wizard' },
    stingers: ST.map
};
