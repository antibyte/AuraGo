// SPDX-License-Identifier: GPL-3.0-only
// "Iron Voltage": hard acid techno, 138 BPM, F minor. A distorted 909 kick, a screaming saw 303
// with a wide-open resonance, a square sub 303 on the off-beats and a compressor that pumps the
// master with every kick.
export default {
    slug: 'iron-voltage',
    title: 'Iron Voltage',
    style: 'Hard acid techno',
    bpm: 138,
    shuffle: 0,
    text: 'Original demo for the RB-338 mode of AuraGo Synth Studio. Hard acid techno, 138 BPM. Press Play in song mode.',

    start: {
        'mix.master': 102,
        'mix.ch1.level': 118, 'mix.ch2.level': 122, 'mix.ch3.level': 120, 'mix.ch4.level': 120,
        'mix.ch1.pan': 58, 'mix.ch2.pan': 64, 'mix.ch3.pan': 72, 'mix.ch4.pan': 64,
        'mix.ch1.send': 28, 'mix.ch3.send': 10,
        'mix.ch1.dist': 1, 'mix.ch4.dist': 1, 'mix.comp': 1, 'mix.pcf': 1,
        't1.wave': 0, 't1.tune': 64, 't1.cutoff': 36, 't1.reso': 112, 't1.envmod': 100, 't1.decay': 40, 't1.accent': 80,
        't2.wave': 1, 't2.tune': 64, 't2.cutoff': 30, 't2.reso': 20, 't2.envmod': 24, 't2.decay': 96, 't2.accent': 40,
        'd909.bd_level': 122, 'd909.bd_tune': 38, 'd909.bd_attack': 96, 'd909.bd_decay': 96,
        'd909.sd_level': 100, 'd909.sd_tone': 80, 'd909.sd_snappy': 100, 'd909.cp_level': 104, 'd909.rs_level': 74,
        'd909.hh_level': 92, 'd909.ch_decay': 22, 'd909.oh_decay': 60, 'd909.cc_level': 100, 'd909.rc_level': 80, 'd909.ac_level': 60,
        'd808.bd_level': 0, 'd808.cp_level': 90, 'd808.ch_level': 90, 'd808.cb_level': 100, 'd808.ac_level': 50,
        'delay.on': 1, 'delay.steps': 3, 'delay.triplet': 1, 'delay.feedback': 64, 'delay.pan': 80,
        'dist.on': 1, 'dist.amount': 84, 'dist.shape': 40,
        'comp.on': 1, 'comp.ratio': 118, 'comp.threshold': 108,
        'pcf.on': 1, 'pcf.freq': 90, 'pcf.q': 72, 'pcf.amt': 0, 'pcf.pattern': 20, 'pcf.dec': 60, 'pcf.mode': 1
    },

    patterns: {
        t1: {
            h1: 'F . F F^> . F . F . G# . F^ . E F .',
            h2: 'F F . F^> . F F . G# . F . C^> . G# .',
            h3: 'F . F . F^> F . F . F^> . F . G#~ G F^>',
            h4: 'F . G# . A# . C^ . D#^ . . F^> F^> . . .',
            pedal: 'F . . . F . . . F . . . F . . .'
        },
        t2: {
            rum: '. . Fv . . . Fv . . . Fv . . . Fv .',
            rumB: '. . Fv . . . Fv . . . Fv . . Gv Fv .'
        },
        d808: {
            clap: { CP: '....x.......x...', CB: '..x...x...x...x.' },
            clap2: { CP: '....x.......x.x.', CB: '.x.x.x.x.x.x.x.x', CH: 'x.x.x.x.x.x.x.x.' }
        },
        d909: {
            kick: { BD: 'x...x...x...x...' },
            hat: { BD: 'x...x...x...x...', OH: '..x...x...x...x.' },
            hard: { BD: 'x...x...x...x...', OH: '..x...x...x...x.', CH: '.x.x.x.x.x.x.x.x', CP: '....x.......x...' },
            hard2: { BD: 'x...x...x...x...', OH: '..x...x...x...x.', CH: '.x.x.x.x.x.x.x.x', CP: '....x.......x...', RS: 'x..x..x..x..x..x' },
            roll: { BD: 'x...x...x...x...', SD: 'x.x.x.x.xxxxxxxx', OH: '..x...x.........' },
            peak: { BD: 'x...x...x...x...', OH: '..x...x...x...x.', CH: '.x.x.x.x.x.x.x.x', CP: '....x.......x...', RC: 'x.x.x.x.x.x.x.x.', CC: 'x...............' },
            tick: { CH: '.x.x.x.x.x.x.x.x', RS: 'x..x..x..x..x..x' }
        }
    },

    arrangement: [
        { name: 'intro', bars: 8, d909: 'kick*4 hat*4' },
        {
            name: 'build', bars: 8, t1: '-*4 h1 h1 h2 h4', t2: 'rum*8', d909: 'hard*7 roll', d808: '-*8',
            auto: [
                { to: 't1.cutoff', points: [[4, 30], [8, 78]], curve: 'exp' },
                { to: 't2.cutoff', points: [[0, 24], [8, 52]] },
                { to: 'mix.ch1.send', points: [[4, 28], [8, 60]] }
            ]
        },
        {
            name: 'drop', bars: 16, t1: 'h1 h2 h1 h3', t2: 'rum rum rum rumB', d909: '(hard*3 hard2)*3 hard*3 roll', d808: 'clap*16',
            set: { 'mix.ch1.send': 24, 't1.envmod': 106 },
            auto: [
                { to: 't1.cutoff', lfo: { center: 58, depth: 24, period: 2, bars: 16 } },
                { to: 't1.reso', points: [[0, 112], [8, 122], [16, 112]] },
                { to: 'd909.bd_decay', points: [[0, 96], [8, 112], [16, 96]] }
            ]
        },
        {
            name: 'break', bars: 8, t1: 'pedal*4 h4 h4 h2 h4', t2: '-*8', d909: '-*4 tick*4', d808: '-*8',
            set: { 'pcf.amt': 80, 'mix.ch1.send': 70, 'delay.feedback': 86 },
            auto: [
                { to: 't1.cutoff', points: [[0, 20], [4, 70], [8, 120]], curve: 'exp' },
                { to: 't1.reso', points: [[0, 100], [8, 126]] },
                { to: 'pcf.freq', points: [[0, 100], [8, 30]] },
                { to: 'dist.amount', points: [[0, 84], [8, 110]] }
            ]
        },
        {
            name: 'drop 2', bars: 16, t1: 'h3 h1 h2 h4', t2: 'rumB rum rumB rum', d909: '(peak hard hard2 hard)*3 peak hard hard2 roll', d808: 'clap2*16',
            set: { 'pcf.amt': 0, 'mix.ch1.send': 24, 'delay.feedback': 64, 'dist.amount': 84, 't1.decay': 34 },
            auto: [
                { to: 't1.cutoff', lfo: { center: 66, depth: 28, period: 3, bars: 16, wave: 'tri' } },
                { to: 'comp.ratio', points: [[0, 118], [8, 127], [16, 118]] },
                { to: 'd909.bd_tune', points: [[0, 38], [8, 30], [16, 38]] }
            ]
        },
        {
            name: 'peak', bars: 8, t1: 'h1 h2 h3 h4', t2: 'rumB*8', d909: 'peak*7 roll', d808: 'clap2*8',
            set: { 't1.envmod': 110 },
            auto: [{ to: 't1.cutoff', points: [[0, 50], [8, 116]] }, { to: 't1.reso', points: [[0, 116], [8, 126]] }]
        },
        {
            name: 'outro', bars: 8, t1: 'h1 pedal*3 -*4', t2: 'rum*4 -*4', d909: 'hard hat*3 kick*4', d808: '-*8',
            auto: [
                { to: 't1.cutoff', points: [[0, 60], [4, 20]] },
                { to: 'mix.ch4.level', points: [[4, 120], [8, 70]] },
                { to: 'mix.ch2.level', points: [[0, 122], [4, 100]] }
            ]
        }
    ]
};
