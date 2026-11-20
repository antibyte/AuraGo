// SPDX-License-Identifier: GPL-3.0-only
// "Midnight Platform": Detroit-style deep techno, 128 BPM, D minor. A warm square-wave 303 rolls
// on the off-beats, a second 303 plays the melodic line into a long delay, the 909 stays tight.
export default {
    slug: 'midnight-platform',
    title: 'Midnight Platform',
    style: 'Deep techno',
    bpm: 128,
    shuffle: 10,
    text: 'Original demo for the RB-338 mode of AuraGo Synth Studio. Deep techno, 128 BPM. Press Play in song mode.',

    start: {
        'mix.master': 107,
        'mix.ch1.level': 118, 'mix.ch2.level': 114, 'mix.ch3.level': 124, 'mix.ch4.level': 120,
        'mix.ch1.pan': 64, 'mix.ch2.pan': 74, 'mix.ch3.pan': 52, 'mix.ch4.pan': 64,
        'mix.ch2.send': 72, 'mix.ch3.send': 24, 'mix.ch1.dist': 1, 'mix.ch2.dist': 1, 'mix.ch3.dist': 1, 'mix.ch4.dist': 1,
        'mix.comp': 1, 'mix.pcf': 2,
        't1.wave': 1, 't1.tune': 64, 't1.cutoff': 34, 't1.reso': 56, 't1.envmod': 46, 't1.decay': 84, 't1.accent': 58,
        't2.wave': 0, 't2.tune': 64, 't2.cutoff': 54, 't2.reso': 72, 't2.envmod': 62, 't2.decay': 64, 't2.accent': 70,
        'd909.bd_level': 116, 'd909.bd_tune': 48, 'd909.bd_attack': 44, 'd909.bd_decay': 84,
        'd909.sd_level': 80, 'd909.rs_level': 84, 'd909.cp_level': 74, 'd909.hh_level': 72,
        'd909.ch_decay': 18, 'd909.oh_decay': 38, 'd909.cc_level': 80, 'd909.ac_level': 40,
        'd808.slot3_switch': 1, 'd808.slot4_switch': 1, 'd808.slot5_switch': 1,
        'd808.lt_level': 100, 'd808.mt_level': 100, 'd808.ht_level': 100, 'd808.lt_tuning': 52, 'd808.mt_tuning': 64, 'd808.ht_tuning': 78,
        'd808.cb_level': 90, 'd808.ac_level': 36, 'd808.bd_level': 0,
        'delay.on': 1, 'delay.steps': 3, 'delay.triplet': 1, 'delay.feedback': 88, 'delay.pan': 76,
        'dist.on': 1, 'dist.amount': 76, 'dist.shape': 0,
        'comp.on': 1, 'comp.ratio': 70, 'comp.threshold': 84,
        'pcf.on': 1, 'pcf.freq': 54, 'pcf.q': 60, 'pcf.amt': 0, 'pcf.pattern': 12, 'pcf.dec': 80, 'pcf.mode': 1
    },

    patterns: {
        t1: {
            roll: '. . D . . . D . . . D . . . F .',
            rollB: '. . D . . D . . . . D . . . A~ G',
            rollC: '. . D^ . . . D . . . D . D . C .',
            deep: 'D . . . . . . . . . . . F . . .',
            turn: '. . D . . . D . . . F . . . G A~'
        },
        t2: {
            m1: 'D^ . . F^ . . A^ . . . G^ . F^ . . .',
            m2: 'D^ . . F^ . . A^ . . C\'^ . . A^ . G^ .',
            m3: 'A^ . . . G^ . F^ . . . D^ . . . . .',
            stab: 'D^> . . . . . F^> . . . . . A^> . . .'
        },
        d808: {
            perc1: { LT: '..x....x..x.....', MT: '.....x.......x..', HT: '....x...........', CB: '...x..x....x..x.', AC: '...x.......x....' },
            perc2: { LT: 'x..x..x.....x...', MT: '.....x...x......', HT: '..x.......x..x..', AC: 'x...........x...' }
        },
        d909: {
            k4: { BD: 'x...x...x...x...' },
            h1: { BD: 'x...x...x...x...', OH: '..x...x...x...x.' },
            groove: { BD: 'x...x...x...x...', OH: '..x...x...x...x.', CH: '...x...x...x...x', CP: '....x.......x...' },
            groove2: { BD: 'x...x...x...x...', OH: '..x...x...x...x.', CH: '...x...x...x...x', CP: '....x.......x...', RS: '.x..x...x..x..x.' },
            fill: { BD: 'x...x...x...x...', OH: '..x...x.........', CH: '...x...x........', SD: '..........x.xxxx', CP: '....x...........' },
            tick: { CH: '...x...x...x...x', RS: '..x.......x.....' },
            crash: { BD: 'x...x...x...x...', CC: 'x...............', OH: '..x...x...x...x.', CH: '...x...x...x...x', CP: '....x.......x...' }
        }
    },

    arrangement: [
        { name: 'intro', bars: 8, d909: 'k4*4 h1*4' },
        {
            name: 'rolling', bars: 8, t1: 'roll roll roll rollB roll roll rollB rollC', d909: 'groove*7 fill',
            auto: [{ to: 't1.cutoff', points: [[0, 30], [8, 52]] }]
        },
        {
            name: 'main A', bars: 16, t1: 'roll rollB roll rollC', t2: '-*4 m1 m1 m2 m1 m1 m1 m2 m3 m1 m1 m2 m3',
            d909: '(groove*3 groove2)*3 groove*3 fill', d808: '-*8 perc1*4 perc2*4',
            set: { 't1.envmod': 52 },
            auto: [
                { to: 't1.cutoff', lfo: { center: 52, depth: 14, period: 8, bars: 16 } },
                { to: 't2.cutoff', points: [[4, 46], [16, 96]], curve: 'smooth' },
                { to: 'delay.feedback', points: [[12, 88], [15, 104], [16, 88]] }
            ]
        },
        {
            name: 'break', bars: 8, t1: 'deep*8', t2: 'stab*8', d909: '-*4 tick*4', d808: '-*4 perc1*4',
            set: { 'mix.ch2.send': 100, 't2.decay': 100, 't2.envmod': 80, 'pcf.amt': 70 },
            auto: [
                { to: 'pcf.freq', points: [[0, 34], [4, 100], [8, 44]], curve: 'smooth' },
                { to: 'pcf.q', points: [[0, 40], [8, 110]] },
                { to: 't2.cutoff', lfo: { center: 70, depth: 40, period: 4, bars: 8 } },
                { to: 'delay.feedback', points: [[0, 92], [6, 110], [8, 94]] }
            ]
        },
        {
            name: 'main B', bars: 16, t1: 'rollB rollC roll turn', t2: 'm2 m1 m2 m3',
            d909: '(crash groove groove2 groove groove2 groove*2 fill)*2', d808: 'perc1 perc2',
            set: { 'pcf.amt': 0, 'mix.ch2.send': 78, 't2.decay': 64, 't1.envmod': 56, 'delay.feedback': 90 },
            auto: [
                { to: 't1.cutoff', lfo: { center: 56, depth: 16, period: 4, bars: 16 } },
                { to: 't2.cutoff', lfo: { center: 82, depth: 24, period: 8, bars: 16, phase: 0.25 } },
                { to: 't2.reso', points: [[0, 72], [16, 98]] }
            ]
        },
        {
            name: 'outro', bars: 8, t1: 'roll roll deep*2 -*4', t2: 'm1 m3 -*6', d909: 'groove h1*3 k4*4', d808: 'perc2 -*7',
            auto: [
                { to: 't1.cutoff', points: [[0, 60], [4, 24]] },
                { to: 'mix.ch4.level', points: [[4, 120], [8, 60]] },
                { to: 'delay.feedback', points: [[0, 92], [8, 110]] }
            ]
        }
    ]
};
