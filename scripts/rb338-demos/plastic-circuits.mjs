// SPDX-License-Identifier: GPL-3.0-only
// "Plastic Circuits": 808 electro breakbeat, 118 BPM, E minor. Syncopated 808 kick and snare
// with a lightly swung 909 hi-hat, a square 303 robot bass and a saw 303 lead, both through the
// distortion.
export default {
    slug: 'plastic-circuits',
    title: 'Plastic Circuits',
    style: 'Electro breakbeat',
    bpm: 118,
    shuffle: 28,
    text: 'Original demo for the RB-338 mode of AuraGo Synth Studio. Electro breakbeat, 118 BPM. Press Play in song mode.',

    start: {
        'mix.master': 104,
        'mix.ch1.level': 120, 'mix.ch2.level': 112, 'mix.ch3.level': 120, 'mix.ch4.level': 122,
        'mix.ch1.pan': 56, 'mix.ch2.pan': 76, 'mix.ch3.pan': 64, 'mix.ch4.pan': 70,
        'mix.ch2.send': 60, 'mix.ch3.send': 8, 'mix.ch4.send': 0,
        'mix.ch1.dist': 1, 'mix.ch3.dist': 1, 'mix.comp': 1, 'mix.pcf': 2,
        't1.wave': 1, 't1.tune': 64, 't1.cutoff': 62, 't1.reso': 70, 't1.envmod': 40, 't1.decay': 38, 't1.accent': 90,
        't2.wave': 0, 't2.tune': 64, 't2.cutoff': 78, 't2.reso': 88, 't2.envmod': 70, 't2.decay': 44, 't2.accent': 96,
        'd808.bd_level': 126, 'd808.bd_tone': 40, 'd808.bd_decay': 72, 'd808.sd_level': 100, 'd808.sd_tone': 76, 'd808.sd_snappy': 88,
        'd808.cp_level': 100, 'd808.cb_level': 90, 'd808.ac_level': 60,
        'd808.lt_level': 104, 'd808.mt_level': 104, 'd808.ht_level': 104, 'd808.lt_tuning': 50, 'd808.mt_tuning': 66, 'd808.ht_tuning': 84,
        'd808.ch_level': 80, 'd808.oh_level': 86, 'd808.oh_decay': 44, 'd808.cy_level': 88, 'd808.cy_decay': 90,
        'd909.bd_level': 0, 'd909.hh_level': 96, 'd909.ch_decay': 20, 'd909.oh_decay': 36, 'd909.cc_level': 96, 'd909.rc_level': 70,
        'delay.on': 1, 'delay.steps': 3, 'delay.triplet': 1, 'delay.feedback': 76, 'delay.pan': 82,
        'dist.on': 1, 'dist.amount': 52, 'dist.shape': 24,
        'comp.on': 1, 'comp.ratio': 78, 'comp.threshold': 90,
        'pcf.on': 1, 'pcf.freq': 70, 'pcf.q': 62, 'pcf.amt': 0, 'pcf.pattern': 30, 'pcf.dec': 70, 'pcf.mode': 0
    },

    patterns: {
        t1: {
            robot: 'E . E . . E^ . . E . G . . A . B~',
            robotB: 'E . E . . E^ . E . . G . A . G .',
            robotC: 'E . . E^> . . E . D . . E . B . .',
            drone: 'E . . . . . . . E . . . . . . .'
        },
        t2: {
            leadA: 'B^ . . E^ . . G^ . B^ . . A^~ G^ . . .',
            leadB: 'E^ G^ . B^ . . C\'^ . B^ . G^ . E^ . . .',
            leadC: 'G^~ B^ . . E^> . . . G^ . B^~ C\'^ . . . .',
            arp: 'E^ B^ E^ G^ B^ G^ E^ B^ E^ B^ E^ G^ B^ G^ E^ B^'
        },
        d808: {
            bk1: { BD: 'x.....x...x..x..', SD: '....x.......x...', CP: '....x.......x..x', CB: '..x.......x.....' },
            bk2: { BD: 'x..x..x...x...x.', SD: '....x.......x..x', CP: '....x.......x...', CB: '..x...x...x...x.', AC: 'x.......x.......' },
            bk3: { BD: 'x.....x.x.x..x..', SD: '....x.......x...', CP: '....x...........', OH: '.............x..', LT: '...........x....', AC: 'x...........x...' },
            fill: { BD: 'x.....x.........', SD: '....x...x.x.xxxx', LT: '........x.x.....', MT: '..........x.x...', HT: '............x.x.', CP: '....x...........' },
            toms: { LT: 'x..x..x.....x...', MT: '....x..x......x.', HT: '..x...........x.', CB: '....x.......x...', AC: 'x...x...x...x...' },
            hold: { BD: 'x...............', CY: 'x...............' }
        },
        d909: {
            hat: { CH: 'x.x.x.x.x.x.x.x.', OH: '.......x........' },
            hat2: { CH: 'x.xxx.xxx.xxx.x.', OH: '.............x..', AC: 'x...x...x...x...' },
            ride: { RC: 'x.x.x.x.x.x.x.x.', CC: 'x...............' }
        }
    },

    arrangement: [
        { name: 'intro', bars: 8, d808: 'bk1*3 bk2 bk1*3 fill', d909: '-*4 hat*4' },
        {
            name: 'robot', bars: 8, t1: 'robot robot robotB robot robot robot robotB robotC', d808: 'bk2 bk1 bk2 bk3 bk2 bk1 bk2 fill', d909: 'hat*8',
            auto: [{ to: 't1.cutoff', points: [[0, 40], [8, 80]], curve: 'exp' }]
        },
        {
            name: 'lead', bars: 16, t1: 'robot robotB robot robotC', t2: 'leadA leadB leadA leadC',
            d808: '(bk2 bk3 bk2 bk1 bk2 bk3 bk2 fill)*2', d909: 'hat2*16',
            set: { 't1.envmod': 52 },
            auto: [
                { to: 't2.cutoff', lfo: { center: 82, depth: 30, period: 4, bars: 16 } },
                { to: 't1.cutoff', points: [[0, 70], [8, 100], [16, 66]] },
                { to: 'dist.amount', points: [[0, 52], [8, 64], [16, 52]] }
            ]
        },
        {
            name: 'break', bars: 8, t1: 'drone*8', t2: 'arp*8', d808: 'toms*3 hold toms*3 fill', d909: '-*8',
            set: { 'mix.ch2.send': 96, 'delay.feedback': 92, 'pcf.amt': 80 },
            auto: [
                { to: 't2.cutoff', points: [[0, 40], [8, 118]], curve: 'exp' },
                { to: 't2.reso', points: [[0, 70], [8, 120]] },
                { to: 'pcf.freq', points: [[0, 120], [8, 30]] },
                { to: 'd808.sd_tone', points: [[0, 76], [8, 20]] }
            ]
        },
        {
            name: 'full', bars: 16, t1: 'robotB robot robotC robot', t2: 'leadB leadC leadA leadC',
            d808: '(bk2 bk3 bk2 bk3 bk2 bk1 bk3 fill)*2', d909: '(ride*3 hat2)*4',
            set: { 'pcf.amt': 0, 'mix.ch2.send': 62, 'delay.feedback': 80, 't1.decay': 34, 't2.decay': 38 },
            auto: [
                { to: 't2.cutoff', lfo: { center: 90, depth: 28, period: 2, bars: 16 } },
                { to: 't1.cutoff', points: [[0, 90], [16, 110]] },
                { to: 'd808.bd_decay', points: [[0, 72], [8, 90], [16, 72]] }
            ]
        },
        {
            name: 'outro', bars: 8, t1: 'robot robotB drone*6', t2: 'leadA -*7', d808: 'bk1 bk2 bk1 bk1 bk1 bk1 hold -', d909: 'hat*8',
            auto: [
                { to: 't1.cutoff', points: [[0, 80], [8, 20]] },
                { to: 'mix.ch3.level', points: [[4, 120], [8, 70]] }
            ]
        }
    ]
};
