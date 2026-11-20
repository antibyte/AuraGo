// SPDX-License-Identifier: GPL-3.0-only
// "Dub Station Zero": minimal dub techno, 122 BPM, A minor. Very few notes: a deep 303 sub, short
// chord-like stabs that live in the delay, congas and rim clicks. The delay feedback and time and
// the PCF carry the arrangement.
export default {
    slug: 'dub-station-zero',
    title: 'Dub Station Zero',
    style: 'Minimal dub',
    bpm: 122,
    shuffle: 18,
    text: 'Original demo for the RB-338 mode of AuraGo Synth Studio. Minimal dub, 122 BPM. Press Play in song mode.',

    start: {
        'mix.master': 110,
        'mix.ch1.level': 120, 'mix.ch2.level': 114, 'mix.ch3.level': 126, 'mix.ch4.level': 118,
        'mix.ch1.pan': 64, 'mix.ch2.pan': 70, 'mix.ch3.pan': 50, 'mix.ch4.pan': 64,
        'mix.ch2.send': 96, 'mix.ch3.send': 50, 'mix.ch4.send': 18, 'mix.ch1.dist': 1, 'mix.ch2.dist': 1, 'mix.ch3.dist': 1, 'mix.ch4.dist': 1,
        'mix.comp': 1, 'mix.pcf': 2,
        't1.wave': 1, 't1.tune': 64, 't1.cutoff': 28, 't1.reso': 44, 't1.envmod': 30, 't1.decay': 100, 't1.accent': 60,
        't2.wave': 1, 't2.tune': 64, 't2.cutoff': 56, 't2.reso': 64, 't2.envmod': 66, 't2.decay': 36, 't2.accent': 90,
        'd909.bd_level': 112, 'd909.bd_tune': 46, 'd909.bd_attack': 40, 'd909.bd_decay': 78,
        'd909.rs_level': 96, 'd909.hh_level': 66, 'd909.ch_decay': 16, 'd909.oh_decay': 44, 'd909.cp_level': 80, 'd909.ac_level': 30,
        'd808.slot3_switch': 1, 'd808.slot4_switch': 1, 'd808.slot5_switch': 1, 'd808.bd_level': 0,
        'd808.lt_level': 110, 'd808.mt_level': 110, 'd808.ht_level': 110, 'd808.lt_tuning': 46, 'd808.mt_tuning': 62, 'd808.ht_tuning': 80,
        'd808.cb_level': 84, 'd808.ac_level': 40, 'd808.ch_level': 70,
        'delay.on': 1, 'delay.steps': 3, 'delay.triplet': 1, 'delay.feedback': 96, 'delay.pan': 90,
        'dist.on': 1, 'dist.amount': 75, 'dist.shape': 0,
        'comp.on': 1, 'comp.ratio': 74, 'comp.threshold': 86,
        'pcf.on': 1, 'pcf.freq': 60, 'pcf.q': 70, 'pcf.amt': 0, 'pcf.pattern': 8, 'pcf.dec': 90, 'pcf.mode': 1
    },

    patterns: {
        t1: {
            dub1: 'Av . . . . . Av . . . . . . . C .',
            dub2: 'Av . . . . . . . Av . . . . . G .',
            low: 'Av . . . . . . . . . . . . . . .'
        },
        t2: {
            st1: '. . . . A^> . . . . . C\'^ . . . . .',
            st2: '. . E^> . . . . . . . G^> . . . . .',
            st3: '. . . . A^> . . E^ . . . . C\'^ . . .'
        },
        d808: {
            conga1: { LT: '..x....x..x.....', MT: '.....x.......x..', HT: '....x...........', AC: '..x.....x.......' },
            conga2: { LT: 'x.....x.....x...', MT: '..x.........x...', HT: '......x...x.....', CB: '.......x........', AC: 'x.......x.......' },
            clave: { CB: '..x..x....x.....', CH: '.x.x.x.x.x.x.x.x' }
        },
        d909: {
            k4: { BD: 'x...x...x...x...' },
            kh: { BD: 'x...x...x...x...', CH: '.x.x.x.x.x.x.x.x' },
            dub: { BD: 'x...x...x...x...', CH: '.x.x.x.x.x.x.x.x', OH: '..x...x...x...x.', RS: '.....x.......x..' },
            rim: { BD: 'x...x...x...x...', RS: '..x..x....x..x..', CH: '.x.x.x.x.x.x.x.x', CP: '............x...' },
            air: { CH: '.x.x.x.x.x.x.x.x', RS: '..x..x....x.....' }
        }
    },

    arrangement: [
        // the kick alone, then with the hats
        { name: 'intro', bars: 8, d909: 'k4*4 kh*4' },
        {
            name: 'sub', bars: 8, t1: 'dub1 dub1 dub2 dub1', d909: 'kh*4 dub*4', d808: '-*4 conga1*4',
            auto: [{ to: 't1.cutoff', points: [[0, 24], [8, 44]] }]
        },
        {
            name: 'stabs', bars: 16, t1: 'dub1 dub2 dub1 dub2', t2: '-*2 st1 st2 st1 st3 st2 st3 st1 st2 st3 st1 st2 st3 st1 st2', d909: 'dub*7 rim dub*7 rim', d808: 'conga1 conga2 clave conga2',
            auto: [
                { to: 'pcf.amt', points: [[0, 0], [8, 70], [16, 30]] },
                { to: 'pcf.freq', lfo: { center: 66, depth: 38, period: 8, bars: 16 } },
                { to: 'pcf.pattern', hold: [[0, 8], [4, 14], [8, 22], [12, 8]] },
                { to: 'delay.feedback', points: [[0, 96], [10, 100], [14, 112], [16, 98]] },
                { to: 't2.cutoff', points: [[0, 50], [16, 92]] }
            ]
        },
        {
            name: 'dub break', bars: 8, t1: 'low*8', t2: 'st1 -*3 st2 -*3', d909: 'air*8', d808: 'clave*8',
            set: { 'mix.ch2.send': 120, 'delay.steps': 4 },
            auto: [
                { to: 'delay.feedback', points: [[0, 100], [4, 114], [7, 118], [8, 90]] },
                { to: 'pcf.amt', points: [[0, 30], [8, 110]] },
                { to: 'pcf.freq', points: [[0, 100], [4, 24], [8, 90]], curve: 'smooth' },
                { to: 'pcf.q', points: [[0, 70], [8, 118]] },
                { to: 't1.cutoff', points: [[0, 44], [8, 24]] },
                { to: 'delay.pan', lfo: { center: 64, depth: 50, period: 2, bars: 8 } }
            ]
        },
        {
            name: 'return', bars: 16, t1: 'dub2 dub1 dub2 dub1', t2: 'st3 st1 st2 st1 st3 st1 st2 st3 st1 st2 st3 st2 st1 st3 st2 st1', d909: 'rim dub*6 rim dub*7 rim', d808: 'conga2 conga1 clave conga2',
            set: { 'mix.ch2.send': 96, 'delay.steps': 3, 'delay.feedback': 98, 'pcf.amt': 40, 't1.envmod': 36 },
            auto: [
                { to: 'pcf.freq', lfo: { center: 72, depth: 30, period: 4, bars: 16, wave: 'tri' } },
                { to: 'pcf.pattern', hold: [[0, 22], [4, 8], [8, 14], [12, 30]] },
                { to: 'delay.feedback', points: [[0, 98], [8, 108], [16, 96]] },
                { to: 'd909.bd_decay', points: [[0, 78], [16, 62]] }
            ]
        },
        {
            name: 'fade', bars: 8, t1: 'dub1 low*7', t2: 'st2 st1 -*6', d909: 'kh*4 k4*4', d808: 'conga1 -*7',
            set: { 'delay.feedback': 108 },
            auto: [
                { to: 'mix.ch1.level', points: [[0, 120], [8, 50]] },
                { to: 'mix.ch4.level', points: [[4, 118], [8, 60]] },
                { to: 'pcf.amt', points: [[0, 40], [8, 100]] },
                { to: 'pcf.freq', points: [[0, 80], [8, 20]] }
            ]
        }
    ]
};
