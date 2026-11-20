// SPDX-License-Identifier: GPL-3.0-only
// "Acid Sunrise": classic acid house, 124 BPM, A minor. A squelchy sawtooth 303 carries the riff,
// a square 303 plays the high stabs, the 909 keeps four on the floor.
// Notation: 303 steps are notes (C C# D ... B, C' the high C) with ^ octave up, v octave down,
// > accent, ~ slide into the next step, . rest; drums are 16 characters per instrument.
export default {
    slug: 'acid-sunrise',
    title: 'Acid Sunrise',
    style: 'Acid house',
    bpm: 124,
    shuffle: 0,
    text: 'Original demo for the RB-338 mode of AuraGo Synth Studio. Acid house, 124 BPM. Press Play in song mode.',

    // the state at bar 1 (mixer, sound and effect settings)
    start: {
        'mix.master': 106,
        'mix.ch1.level': 112, 'mix.ch2.level': 108, 'mix.ch3.level': 127, 'mix.ch4.level': 120, 'mix.ch1.dist': 1, 'mix.ch2.dist': 1, 'mix.ch3.dist': 1, 'mix.ch4.dist': 1,
        'mix.ch1.pan': 60, 'mix.ch2.pan': 82, 'mix.ch3.pan': 64, 'mix.ch4.pan': 64,
        'mix.ch2.send': 52, 'mix.ch3.send': 12,
        'mix.comp': 1, 'mix.pcf': 2,
        't1.wave': 0, 't1.tune': 64, 't1.cutoff': 22, 't1.reso': 96, 't1.envmod': 92, 't1.decay': 48, 't1.accent': 70,
        't2.wave': 1, 't2.tune': 64, 't2.cutoff': 78, 't2.reso': 58, 't2.envmod': 44, 't2.decay': 28, 't2.accent': 84,
        'd909.bd_level': 118, 'd909.bd_tune': 58, 'd909.bd_attack': 74, 'd909.bd_decay': 66,
        'd909.sd_level': 86, 'd909.sd_tone': 72, 'd909.sd_snappy': 84, 'd909.rs_level': 62, 'd909.cp_level': 100,
        'd909.hh_level': 84, 'd909.ch_decay': 26, 'd909.oh_decay': 52, 'd909.cc_level': 84, 'd909.ac_level': 46,
        'd808.bd_level': 0, 'd808.ch_level': 110, 'd808.cb_level': 110, 'd808.oh_level': 0, 'd808.ac_level': 40,
        'delay.on': 1, 'delay.steps': 3, 'delay.triplet': 1, 'delay.feedback': 74, 'delay.pan': 70,
        'comp.on': 1, 'comp.ratio': 64, 'comp.threshold': 78,
        'pcf.on': 1, 'pcf.freq': 70, 'pcf.q': 44, 'pcf.amt': 30, 'pcf.dec': 90, 'pcf.mode': 1,
        'dist.on': 1, 'dist.amount': 80, 'dist.shape': 0
    },

    patterns: {
        t1: {
            riffA: 'A . . A^> A . G~ A . . C^> . A . D# .',
            riffB: 'A . A . A^> . G . A~ C^ . A> . E~ G .',
            riffC: 'A . A^> . G~ A . C^> D^ . C^ . A> . G A~',
            pedal: 'A . . . A . . . A^> . . . G~ . A .',
            sub: 'Av . . . . . Av . . . . . Gv . . .'
        },
        t2: {
            stab: 'A^ . . A^ . . E^ . . . . . C^ . . .',
            arp: 'A^ . E^ . A^ . C^ . E^ . C^ . A^ . G .',
            long: 'E^~ E^ . . . . . . . . . . . . . .',
            high: 'E^ . G^ . A^ . . . E^ . G^ . C^ . . .'
        },
        d808: {
            hats: { CH: 'x.xxx.xxx.xxx.xx', AC: 'x...x...x...x...' },
            cowbell: { CH: 'x.xxx.xxx.xxx.xx', CB: '.....x.......x..', AC: 'x...x...x...x...' }
        },
        d909: {
            kick: { BD: 'x...x...x...x...' },
            hat: { BD: 'x...x...x...x...', OH: '..x...x...x...x.' },
            groove: { BD: 'x...x...x...x...', OH: '..x...x...x...x.', CH: '.x.x.x.x.x.x.x.x', CP: '....x.......x...' },
            groove2: { BD: 'x...x...x...x...', OH: '..x...x...x...x.', CH: '.x.x.x.x.x.x.x.x', CP: '....x.......x...', RS: '.x..............' },
            fill: { BD: 'x...x...x...x...', SD: '........x.x.xxxx', OH: '..x...x.........', CH: '.x.x.x.x........', CP: '....x...........' },
            tick: { RS: '..x..x..x..x..x.', CH: '.x.x.x.x.x.x.x.x' },
            crash: { BD: 'x...x...x...x...', CC: 'x...............', OH: '..x...x...x...x.', CP: '....x.......x...' }
        }
    },

    arrangement: [
        // intro: kick, then the hats; the riff stays out
        { name: 'intro', bars: 8, d909: 'kick*4 hat*4', d808: '- - - - hats*4' },
        // build: the riff opens its filter
        {
            name: 'build', bars: 8, t1: 'riffA*3 riffB', d909: 'groove*7 fill', d808: 'hats',
            auto: [
                { to: 't1.cutoff', points: [[0, 22], [8, 72]], curve: 'exp' },
                { to: 't1.reso', points: [[0, 96], [8, 108]] }
            ]
        },
        // main: everything plays, the filter breathes
        {
            name: 'main', bars: 16, t1: 'riffA riffA riffB riffA riffA riffB riffA riffC', t2: '-*8 stab*3 arp stab*3 arp',
            d909: '(crash groove*5 groove2 fill)*2', d808: 'cowbell*4 hats*4',
            set: { 't1.envmod': 100, 'mix.ch2.send': 60 },
            auto: [
                { to: 't1.cutoff', lfo: { center: 66, depth: 26, period: 4, bars: 16 } },
                { to: 't2.cutoff', points: [[8, 70], [16, 100]] }
            ]
        },
        // break: only the 303s float in the delay, the PCF sweeps the high line
        {
            name: 'break', bars: 8, t1: 'pedal*4 sub*4', t2: 'long arp high high', d909: '- - - - tick*4',
            set: { 'mix.ch2.send': 96, 'mix.ch1.level': 100, 'delay.feedback': 90, 't1.decay': 70 },
            auto: [
                { to: 't1.cutoff', points: [[0, 40], [8, 30]] },
                { to: 't2.cutoff', points: [[0, 40], [8, 120]], curve: 'exp' },
                { to: 'pcf.freq', points: [[0, 30], [4, 110], [8, 40]], curve: 'smooth' },
                { to: 'pcf.amt', points: [[0, 40], [8, 110]] }
            ]
        },
        // return: harder, the filter opens fully and the acid line gets squeakier
        {
            name: 'return', bars: 16, t1: 'riffA riffB riffA riffC riffA riffB riffA riffC', t2: 'stab*2 arp*2 stab*2 arp*2',
            d909: 'crash groove*6 groove2 groove*7 fill', d808: 'cowbell',
            set: { 'mix.ch1.level': 114, 'mix.ch2.send': 56, 'delay.feedback': 72, 't1.decay': 44, 't1.envmod': 104 },
            auto: [
                { to: 't1.cutoff', points: [[0, 36], [8, 96], [12, 60], [16, 100]] },
                { to: 't1.reso', points: [[0, 104], [16, 114]] },
                { to: 'pcf.amt', points: [[0, 110], [1, 30]] }
            ]
        },
        // outro: the riff drops out, the drums thin out and fade
        {
            name: 'outro', bars: 8, t1: 'riffA riffA pedal pedal sub sub sub sub', t2: '-', d909: 'groove hat*3 kick*4', d808: 'hats*2',
            auto: [
                { to: 't1.cutoff', points: [[0, 80], [8, 18]] },
                { to: 'mix.ch1.level', points: [[4, 112], [8, 40]] },
                { to: 'mix.ch4.level', points: [[4, 112], [8, 70]] }
            ]
        }
    ]
};
