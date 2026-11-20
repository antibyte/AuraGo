// SPDX-License-Identifier: GPL-3.0-only
// "Subwoofer Sunset": Miami bass / hip-hop style 808 groove, 100 BPM, G minor. Heavy shuffle, a
// long 808 kick whose TONE knob walks like a bass line, a square 303 sub and a funky square lead.
export default {
    slug: 'subwoofer-sunset',
    title: 'Subwoofer Sunset',
    style: 'Miami bass',
    bpm: 100,
    shuffle: 92,
    text: 'Original demo for the RB-338 mode of AuraGo Synth Studio. Miami bass groove, 100 BPM. Press Play in song mode.',

    start: {
        'mix.master': 105,
        'mix.ch1.level': 116, 'mix.ch2.level': 112, 'mix.ch3.level': 122, 'mix.ch4.level': 112,
        'mix.ch1.pan': 64, 'mix.ch2.pan': 78, 'mix.ch3.pan': 64, 'mix.ch4.pan': 54,
        'mix.ch2.send': 40, 'mix.ch3.send': 6, 'mix.ch4.send': 10,
        'mix.ch3.dist': 1, 'mix.comp': 1, 'mix.pcf': 0,
        't1.wave': 1, 't1.tune': 64, 't1.cutoff': 26, 't1.reso': 28, 't1.envmod': 22, 't1.decay': 110, 't1.accent': 50,
        't2.wave': 1, 't2.tune': 64, 't2.cutoff': 70, 't2.reso': 54, 't2.envmod': 56, 't2.decay': 52, 't2.accent': 84,
        'd808.bd_level': 127, 'd808.bd_tone': 34, 'd808.bd_decay': 122, 'd808.sd_level': 104, 'd808.sd_tone': 56, 'd808.sd_snappy': 92,
        'd808.cp_level': 112, 'd808.cb_level': 90, 'd808.ac_level': 70, 'd808.ch_level': 84, 'd808.oh_level': 86, 'd808.oh_decay': 72,
        'd808.cy_level': 90, 'd808.cy_decay': 110, 'd808.lt_level': 108, 'd808.mt_level': 108, 'd808.ht_level': 108,
        'd808.lt_tuning': 40, 'd808.mt_tuning': 58, 'd808.ht_tuning': 76,
        'd909.bd_level': 0, 'd909.hh_level': 76, 'd909.ch_decay': 24, 'd909.rs_level': 70, 'd909.cc_level': 90,
        'delay.on': 1, 'delay.steps': 3, 'delay.triplet': 0, 'delay.feedback': 60, 'delay.pan': 74,
        'dist.on': 1, 'dist.amount': 60, 'dist.shape': 12,
        'comp.on': 1, 'comp.ratio': 92, 'comp.threshold': 100,
        'pcf.on': 0
    },

    patterns: {
        t1: {
            sb1: 'G . . G . . . . G . . . A# . C .',
            sb2: 'G . . G . . A# . . . G . . . F .',
            sb3: 'G . . . D . . . G . . . F . D# .',
            hush: 'G . . . . . . . . . . . . . . .'
        },
        t2: {
            ld1: 'D^ . . A#^ . D^ . . C^ . . A#^~ . G^ . .',
            ld2: 'G^ . . G^ . A#^ . . C\'^ . . A#^ . G^ . .',
            ld3: 'F^ . . F^ . G^ . . A#^ . . G^ . F^ . D^'
        },
        d808: {
            // the pattern shuffle flag is on for the whole kit
            beatA: { shuffle: true, steps: { BD: 'x.....x.x.....x.', SD: '....x.......x...', CP: '....x.......x...', CH: 'x.x.x.x.x.x.x.x.' } },
            beatB: { shuffle: true, steps: { BD: 'x.....x..x..x...', SD: '....x.......x...', CP: '....x.......x...', CH: 'x.x.x.x.x.x.x.x.', CB: '.......x.......x' } },
            beatC: { shuffle: true, steps: { BD: 'x..x..x.x...x..x', SD: '....x.......x...', CP: '....x.......x.x.', CH: 'x.x.x.x.x.x.x...', OH: '..............x.', AC: 'x...x...x...x...' } },
            fill: { shuffle: true, steps: { BD: 'x.....x.........', SD: '....x...x.x.xxxx', LT: '........x.x.....', MT: '..........x.x...', HT: '............x.x.', CP: '....x...........' } },
            intro: { shuffle: true, steps: { BD: 'x.....x.........', CH: 'x.x.x.x.x.x.x.x.' } },
            sparse: { shuffle: true, steps: { BD: 'x...............', CB: '..x.....x.......' } },
            hit: { shuffle: true, steps: { BD: 'x...............', CY: 'x...............', CP: 'x...............' } }
        },
        d909: {
            hat: { shuffle: true, steps: { CH: '.x.x.x.x.x.x.x.x', RS: '.....x.......x..' } },
            crash: { shuffle: true, steps: { CC: 'x...............', CH: '.x.x.x.x.x.x.x.x' } }
        }
    },

    arrangement: [
        { name: 'intro', bars: 4, d808: 'intro*3 sparse', d909: '-*4' },
        {
            name: 'verse 1', bars: 12, t1: 'sb1 sb1 sb2 sb1', d808: 'beatA beatA beatB beatA beatA beatA beatB fill beatA beatB beatA fill', d909: '-*4 hat*8',
            auto: [
                // the BD TONE knob walks twice a bar: the long 808 kick is the bass line
                { to: 'd808.bd_tone', hold: [[0, 34], [1.5, 30], [2, 34], [3.5, 40], [4, 34], [5.5, 30], [6, 34], [7.5, 28], [8, 34], [9.5, 40], [10, 34], [11.5, 28]] }
            ]
        },
        {
            name: 'hook', bars: 8, t1: 'sb1 sb2 sb1 sb3', t2: 'ld1 ld2 ld1 ld3', d808: 'beatC beatC beatB beatC beatC beatB beatC fill', d909: 'crash hat*7',
            set: { 'mix.ch2.send': 52 },
            auto: [
                { to: 'd808.bd_tone', hold: [[0, 34], [1.5, 28], [2, 34], [3.5, 42], [4, 34], [5.5, 28], [6, 40], [7.5, 28]] },
                { to: 't2.cutoff', lfo: { center: 78, depth: 20, period: 2, bars: 8 } }
            ]
        },
        {
            name: 'verse 2', bars: 12, t1: 'sb2 sb3 sb2 sb1', t2: '-*4 ld1 -*3 ld3 -*2 ld1', d808: 'beatB beatA beatB beatA beatB beatA beatB fill beatB beatA beatC fill', d909: 'hat*12',
            set: { 'mix.ch2.send': 60, 'delay.feedback': 72 },
            auto: [
                { to: 'd808.bd_tone', hold: [[0, 34], [1.5, 40], [2, 34], [3.5, 28], [4, 38], [5.5, 30], [6, 34], [7.5, 40], [8, 34], [9.5, 28], [10, 34], [11.5, 42]] },
                { to: 'd808.bd_decay', points: [[0, 122], [12, 100]] }
            ]
        },
        {
            name: 'break', bars: 4, t1: 'hush*4', t2: 'ld3 ld2 ld3 ld1', d808: 'sparse*3 hit', d909: '-*4',
            set: { 'mix.ch2.send': 100, 'delay.feedback': 92, 't2.decay': 90 },
            auto: [{ to: 't2.cutoff', points: [[0, 40], [4, 110]], curve: 'exp' }]
        },
        {
            name: 'hook 2', bars: 8, t1: 'sb1 sb2 sb1 sb3', t2: 'ld2 ld1 ld2 ld3', d808: 'beatC beatC beatB beatC beatC beatB beatC fill', d909: 'crash hat*7',
            set: { 'mix.ch2.send': 52, 'delay.feedback': 64, 't2.decay': 52, 'mix.ch1.level': 122 },
            auto: [
                { to: 'd808.bd_tone', hold: [[0, 34], [1.5, 28], [2, 38], [3.5, 28], [4, 34], [5.5, 42], [6, 30], [7.5, 28]] },
                { to: 't2.cutoff', lfo: { center: 90, depth: 20, period: 1, bars: 8 } }
            ]
        },
        {
            name: 'outro', bars: 8, t1: 'sb1 sb2 hush*6', t2: 'ld1 -*7', d808: 'beatA beatB beatA sparse*3 hit -', d909: 'hat*2 -*6',
            auto: [
                { to: 'mix.ch3.level', points: [[4, 122], [8, 70]] },
                { to: 'mix.ch1.level', points: [[2, 122], [8, 60]] }
            ]
        }
    ]
};
