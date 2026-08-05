(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createAudioMusic = function (ctx) {
        const audio = () => ctx.audio();
        const beep = (...args) => ctx.beep(...args);
        const schedNoise = (...args) => ctx.schedNoise(...args);
                const MusicEngine = {
            nodes: [], masterGain: null, playing: null, loopId: 0, tempoMult: 1, stopped: false, intensity: 5,
            themes: {
                title: {
                    bpm: 120,
                    bass: { wave: 'triangle', vol: 0.06, notes: [{ f: 131, d: 2 }, { f: 0, d: 2 }, { f: 156, d: 2 }, { f: 0, d: 2 }, { f: 131, d: 2 }, { f: 0, d: 1 }, { f: 117, d: 1 }, { f: 0, d: 2 }, { f: 156, d: 2 }, { f: 0, d: 1 }, { f: 131, d: 1 }, { f: 0, d: 2 }] },
                    lead: { wave: 'sine', vol: 0.08, notes: [{ f: 262, d: 1 }, { f: 233, d: 1 }, { f: 311, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 2 }, { f: 233, d: 2 }, { f: 349, d: 1 }, { f: 311, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 1 }, { f: 233, d: 2 }, { f: 262, d: 2 }] },
                    harmony: { wave: 'sine', vol: 0.04, notes: [{ f: 311, d: 2 }, { f: 349, d: 2 }, { f: 262, d: 2 }, { f: 294, d: 2 }, { f: 349, d: 2 }, { f: 311, d: 2 }, { f: 262, d: 2 }, { f: 233, d: 2 }] },
                    arpeggio: { wave: 'square', vol: 0.02, notes: [{ f: 262, d: 0.5 }, { f: 311, d: 0.5 }, { f: 349, d: 0.5 }, { f: 262, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 233, d: 0.5 }, { f: 262, d: 0.5 }, { f: 311, d: 0.5 }, { f: 349, d: 0.5 }, { f: 262, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 233, d: 0.5 }] },
                    percussion: { vol: 0.04, notes: [{ f: -1, d: 1 }, { f: 0, d: 1 }, { f: -2, d: 0.5 }, { f: 0, d: 0.5 }, { f: -1, d: 1 }, { f: 0, d: 1 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 1 }, { f: 0, d: 1 }, { f: -2, d: 0.5 }, { f: 0, d: 0.5 }, { f: -1, d: 1 }, { f: 0, d: 1 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                gameplay: {
                    bpm: 140,
                    bass: { wave: 'triangle', vol: 0.07, notes: [{ f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 0, d: 0.5 }, { f: 156, d: 0.5 }, { f: 156, d: 0.5 }, { f: 156, d: 0.5 }, { f: 0, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 0, d: 0.5 }, { f: 117, d: 0.5 }, { f: 117, d: 0.5 }, { f: 117, d: 0.5 }, { f: 0, d: 0.5 }, { f: 131, d: 0.5 }, { f: 156, d: 0.5 }, { f: 175, d: 0.5 }, { f: 0, d: 0.5 }, { f: 156, d: 0.5 }, { f: 175, d: 0.5 }, { f: 196, d: 0.5 }, { f: 0, d: 0.5 }, { f: 131, d: 0.5 }, { f: 147, d: 0.5 }, { f: 175, d: 0.5 }, { f: 0, d: 0.5 }, { f: 117, d: 0.5 }, { f: 131, d: 0.5 }, { f: 156, d: 0.5 }, { f: 0, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.05, notes: [{ f: 262, d: 0.5 }, { f: 311, d: 0.5 }, { f: 392, d: 0.5 }, { f: 262, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 233, d: 0.5 }, { f: 207, d: 0.5 }, { f: 262, d: 0.5 }, { f: 311, d: 0.5 }, { f: 207, d: 0.5 }, { f: 196, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 196, d: 0.5 }, { f: 349, d: 0.5 }, { f: 392, d: 0.5 }, { f: 440, d: 1 }, { f: 392, d: 0.5 }, { f: 349, d: 0.5 }, { f: 440, d: 1 }, { f: 392, d: 0.5 }, { f: 349, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 392, d: 1 }, { f: 294, d: 0.5 }, { f: 233, d: 0.5 }, { f: 262, d: 1 }, { f: 233, d: 0.5 }, { f: 196, d: 0.5 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 262, d: 1 }, { f: 311, d: 1 }, { f: 233, d: 1 }, { f: 294, d: 1 }, { f: 207, d: 1 }, { f: 262, d: 1 }, { f: 196, d: 1 }, { f: 233, d: 1 }, { f: 349, d: 1 }, { f: 392, d: 1 }, { f: 440, d: 1 }, { f: 392, d: 1 }, { f: 294, d: 1 }, { f: 349, d: 1 }, { f: 262, d: 1 }, { f: 233, d: 1 }] },
                    arpeggio: { wave: 'sine', vol: 0.02, notes: [{ f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 156, d: 0.25 }, { f: 233, d: 0.25 }, { f: 311, d: 0.25 }, { f: 233, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 117, d: 0.25 }, { f: 175, d: 0.25 }, { f: 233, d: 0.25 }, { f: 175, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 156, d: 0.25 }, { f: 233, d: 0.25 }, { f: 311, d: 0.25 }, { f: 233, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 117, d: 0.25 }, { f: 175, d: 0.25 }, { f: 233, d: 0.25 }, { f: 175, d: 0.25 }] },
                    percussion: { vol: 0.04, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                boss: {
                    bpm: 160,
                    bass: { wave: 'sawtooth', vol: 0.05, notes: [{ f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 110, d: 1 }, { f: 123, d: 1 }, { f: 131, d: 1 }, { f: 147, d: 1 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.04, notes: [{ f: 220, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 220, d: 0.5 }, { f: 247, d: 0.5 }, { f: 294, d: 0.5 }, { f: 330, d: 0.5 }, { f: 247, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 440, d: 0.5 }, { f: 262, d: 0.5 }, { f: 247, d: 0.5 }, { f: 294, d: 0.5 }, { f: 330, d: 0.5 }, { f: 247, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 330, d: 0.5 }, { f: 294, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 330, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 220, d: 0.5 }, { f: 262, d: 1 }, { f: 330, d: 1 }, { f: 220, d: 1 }, { f: 247, d: 1 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 165, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }, { f: 247, d: 1 }, { f: 262, d: 1 }, { f: 165, d: 1 }, { f: 247, d: 1 }, { f: 294, d: 1 }, { f: 220, d: 1 }, { f: 262, d: 1 }, { f: 330, d: 1 }, { f: 440, d: 1 }, { f: 294, d: 1 }, { f: 330, d: 1 }, { f: 220, d: 1 }, { f: 247, d: 1 }] },
                    arpeggio: { wave: 'sawtooth', vol: 0.02, notes: [{ f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }] },
                    percussion: { vol: 0.05, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                miniboss: {
                    bpm: 150,
                    bass: { wave: 'sawtooth', vol: 0.06, notes: [{ f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 175, d: 0.5 }, { f: 175, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.04, notes: [{ f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 440, d: 0.5 }, { f: 294, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 262, d: 0.5 }, { f: 349, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 349, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 440, d: 0.5 }, { f: 330, d: 0.5 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 220, d: 1 }, { f: 262, d: 1 }, { f: 294, d: 1 }, { f: 330, d: 1 }, { f: 349, d: 1 }, { f: 262, d: 1 }, { f: 294, d: 1 }, { f: 220, d: 1 }] },
                    arpeggio: { wave: 'sawtooth', vol: 0.015, notes: [{ f: 147, d: 0.25 }, { f: 220, d: 0.25 }, { f: 294, d: 0.25 }, { f: 220, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }, { f: 147, d: 0.25 }, { f: 220, d: 0.25 }, { f: 294, d: 0.25 }, { f: 220, d: 0.25 }, { f: 175, d: 0.25 }, { f: 262, d: 0.25 }, { f: 349, d: 0.25 }, { f: 262, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }] },
                    percussion: { vol: 0.05, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                gameover: {
                    bpm: 100,
                    bass: { wave: 'triangle', vol: 0.06, notes: [{ f: 131, d: 1 }, { f: 117, d: 1 }, { f: 104, d: 1 }, { f: 98, d: 1 }, { f: 87, d: 1 }, { f: 78, d: 2 }, { f: 131, d: 0.5 }, { f: 0, d: 0.5 }, { f: 117, d: 0.5 }, { f: 0, d: 0.5 }, { f: 104, d: 1 }, { f: 78, d: 2 }] },
                    lead: { wave: 'sine', vol: 0.1, notes: [{ f: 262, d: 1 }, { f: 233, d: 1 }, { f: 207, d: 1 }, { f: 196, d: 1 }, { f: 175, d: 1 }, { f: 156, d: 2 }, { f: 262, d: 0.5 }, { f: 233, d: 0.5 }, { f: 207, d: 0.5 }, { f: 196, d: 0.5 }, { f: 175, d: 1 }, { f: 156, d: 2 }] },
                    harmony: { wave: 'sine', vol: 0.04, notes: [{ f: 311, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 1 }, { f: 233, d: 1 }, { f: 207, d: 1 }, { f: 0, d: 2 }, { f: 311, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 1 }, { f: 233, d: 1 }, { f: 207, d: 1 }, { f: 0, d: 2 }] },
                    percussion: { vol: 0.03, notes: [{ f: -1, d: 1 }, { f: 0, d: 2 }, { f: -1, d: 1 }, { f: 0, d: 3 }, { f: -1, d: 1 }, { f: 0, d: 5 }] }
                },
                challenge: {
                    bpm: 170,
                    bass: { wave: 'sawtooth', vol: 0.06, notes: [{ f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 165, d: 0.5 }, { f: 165, d: 0.5 }, { f: 98, d: 0.25 }, { f: 131, d: 0.25 }, { f: 98, d: 0.25 }, { f: 131, d: 0.25 }, { f: 110, d: 0.25 }, { f: 147, d: 0.25 }, { f: 110, d: 0.25 }, { f: 147, d: 0.25 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 165, d: 0.5 }, { f: 165, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.05, notes: [{ f: 196, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 440, d: 0.5 }, { f: 392, d: 0.5 }, { f: 330, d: 0.5 }, { f: 262, d: 0.5 }, { f: 220, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 349, d: 0.5 }, { f: 294, d: 0.5 }, { f: 262, d: 0.25 }, { f: 330, d: 0.25 }, { f: 392, d: 0.25 }, { f: 523, d: 0.25 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 659, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 349, d: 0.5 }, { f: 294, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 196, d: 1 }, { f: 262, d: 1 }, { f: 330, d: 1 }, { f: 392, d: 1 }, { f: 440, d: 1 }, { f: 349, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 1 }, { f: 330, d: 1 }, { f: 392, d: 1 }, { f: 440, d: 1 }, { f: 523, d: 1 }, { f: 659, d: 1 }, { f: 523, d: 1 }, { f: 440, d: 1 }, { f: 349, d: 1 }] },
                    arpeggio: { wave: 'square', vol: 0.02, notes: [{ f: 98, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 110, d: 0.25 }, { f: 147, d: 0.25 }, { f: 220, d: 0.25 }, { f: 294, d: 0.25 }, { f: 131, d: 0.25 }, { f: 165, d: 0.25 }, { f: 262, d: 0.25 }, { f: 330, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 294, d: 0.25 }, { f: 392, d: 0.25 }, { f: 98, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 110, d: 0.25 }, { f: 147, d: 0.25 }, { f: 220, d: 0.25 }, { f: 294, d: 0.25 }, { f: 131, d: 0.25 }, { f: 165, d: 0.25 }, { f: 262, d: 0.25 }, { f: 330, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 294, d: 0.25 }, { f: 392, d: 0.25 }] },
                    percussion: { vol: 0.05, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.25 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.25 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                deep_boss: {
                    bpm: 170,
                    bass: { wave: 'sawtooth', vol: 0.06, notes: [{ f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 87, d: 0.5 }, { f: 87, d: 0.5 }, { f: 87, d: 0.5 }, { f: 87, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 82, d: 1 }, { f: 98, d: 1 }, { f: 110, d: 1 }, { f: 131, d: 1 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 82, d: 0.5 }, { f: 82, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.04, notes: [{ f: 196, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 196, d: 0.5 }, { f: 220, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 220, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 392, d: 0.5 }, { f: 233, d: 0.5 }, { f: 220, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 220, d: 0.5 }, { f: 392, d: 0.5 }, { f: 466, d: 0.5 }, { f: 392, d: 0.5 }, { f: 294, d: 0.5 }, { f: 262, d: 0.5 }, { f: 392, d: 0.5 }, { f: 466, d: 0.5 }, { f: 392, d: 0.5 }, { f: 294, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 196, d: 0.5 }, { f: 233, d: 1 }, { f: 294, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }] },
                    harmony: { wave: 'sine', vol: 0.025, notes: [{ f: 147, d: 1 }, { f: 175, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }, { f: 233, d: 1 }, { f: 147, d: 1 }, { f: 220, d: 1 }, { f: 262, d: 1 }, { f: 196, d: 1 }, { f: 233, d: 1 }, { f: 294, d: 1 }, { f: 392, d: 1 }, { f: 262, d: 1 }, { f: 294, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }] },
                    arpeggio: { wave: 'sawtooth', vol: 0.015, notes: [{ f: 98, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 147, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 98, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 147, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }] },
                    percussion: { vol: 0.06, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                victory: {
                    bpm: 180,
                    bass: { wave: 'triangle', vol: 0.08, notes: [{f:131,d:0.5},{f:0,d:0.5},{f:165,d:0.5},{f:0,d:0.5},{f:196,d:0.5},{f:0,d:0.5},{f:262,d:1},{f:220,d:0.5},{f:0,d:0.5},{f:262,d:0.5},{f:0,d:0.5},{f:330,d:0.5},{f:0,d:0.5},{f:392,d:1}] },
                    lead: { wave: 'sine', vol: 0.14, notes: [{f:523,d:0.5},{f:659,d:0.5},{f:784,d:0.5},{f:1047,d:1.5},{f:880,d:0.5},{f:1047,d:0.5},{f:1175,d:0.5},{f:1397,d:1.5}] },
                    harmony: { wave: 'sine', vol: 0.07, notes: [{f:392,d:1},{f:494,d:1},{f:587,d:1},{f:784,d:2},{f:659,d:1},{f:784,d:1},{f:880,d:1},{f:1047,d:2}] },
                    arpeggio: { wave: 'triangle', vol: 0.04, notes: [{f:262,d:0.25},{f:330,d:0.25},{f:392,d:0.25},{f:523,d:0.25},{f:330,d:0.25},{f:392,d:0.25},{f:523,d:0.25},{f:659,d:0.25},{f:440,d:0.25},{f:523,d:0.25},{f:659,d:0.25},{f:880,d:0.25},{f:523,d:0.25},{f:659,d:0.25},{f:880,d:0.25},{f:1047,d:0.25}] },
                    percussion: { vol: 0.07, notes: [{f:-1,d:0.5},{f:-2,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5},{f:-3,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5},{f:-2,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5},{f:-3,d:0.5},{f:-1,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5}] }
                },
                gauntlet: {
                    bpm: 155,
                    bass: { wave: 'sawtooth', vol: 0.07, notes: [{ f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.05, notes: [{ f: 196, d: 0.5 }, { f: 247, d: 0.5 }, { f: 294, d: 0.5 }, { f: 392, d: 0.5 }, { f: 330, d: 0.5 }, { f: 262, d: 0.5 }, { f: 220, d: 0.5 }, { f: 196, d: 0.5 }] },
                    harmony: { wave: 'triangle', vol: 0.03, notes: [{ f: 147, d: 1 }, { f: 175, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }] },
                    arpeggio: { wave: 'sawtooth', vol: 0.02, notes: [{ f: 98, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 147, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }] },
                    percussion: { vol: 0.05, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -3, d: 0.5 }] }
                },
                hyperdrive: {
                    bpm: 168,
                    bass: { wave: 'sawtooth', vol: 0.075, notes: [{ f: 110, d: 0.25 }, { f: 110, d: 0.25 }, { f: 131, d: 0.25 }, { f: 131, d: 0.25 }, { f: 147, d: 0.25 }, { f: 147, d: 0.25 }, { f: 165, d: 0.25 }, { f: 165, d: 0.25 }] },
                    lead: { wave: 'square', vol: 0.055, notes: [{ f: 440, d: 0.25 }, { f: 523, d: 0.25 }, { f: 659, d: 0.25 }, { f: 784, d: 0.5 }, { f: 659, d: 0.25 }, { f: 523, d: 0.25 }, { f: 440, d: 0.25 }, { f: 392, d: 0.25 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 220, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }] },
                    arpeggio: { wave: 'square', vol: 0.025, notes: [{ f: 220, d: 0.125 }, { f: 330, d: 0.125 }, { f: 440, d: 0.125 }, { f: 330, d: 0.125 }, { f: 262, d: 0.125 }, { f: 392, d: 0.125 }, { f: 523, d: 0.125 }, { f: 392, d: 0.125 }] },
                    percussion: { vol: 0.055, notes: [{ f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.25 }, { f: -3, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }] }
                },
                mirror: {
                    bpm: 132,
                    bass: { wave: 'triangle', vol: 0.06, notes: [{ f: 131, d: 1 }, { f: 117, d: 1 }, { f: 131, d: 1 }, { f: 147, d: 1 }] },
                    lead: { wave: 'sine', vol: 0.07, notes: [{ f: 523, d: 0.5 }, { f: 659, d: 0.5 }, { f: 784, d: 0.5 }, { f: 659, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 659, d: 0.5 }] },
                    harmony: { wave: 'sine', vol: 0.035, notes: [{ f: 262, d: 1 }, { f: 330, d: 1 }, { f: 392, d: 1 }, { f: 330, d: 1 }] },
                    arpeggio: { wave: 'triangle', vol: 0.02, notes: [{ f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 523, d: 0.5 }, { f: 392, d: 0.5 }, { f: 330, d: 0.5 }, { f: 262, d: 0.5 }, { f: 220, d: 0.5 }] },
                    percussion: { vol: 0.035, notes: [{ f: -1, d: 1 }, { f: 0, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 1 }, { f: 0, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 1 }, { f: 0, d: 0.5 }] }
                }
            },
            play(theme) {
                if (this.playing === theme && !this.stopped) return;
                const prevTheme = this.playing;
                const prevGain = this.masterGain;
                this.stop(); this.playing = theme; this.stopped = false;
                const a = audio(); if (!a) return;
                if (prevGain) { prevGain.gain.linearRampToValueAtTime(0, a.currentTime + 0.3); setTimeout(() => { try { prevGain.disconnect(); } catch (_) {} }, 350); }
                if (prevTheme && prevTheme !== theme && !ctx.G.muted) {
                    const stingerVol = ctx.G.vol * 0.15;
                    if (theme === 'boss' || theme === 'miniboss' || theme === 'deep_boss') {
                        beep('sawtooth', 220, 110, 0.3, stingerVol);
                        setTimeout(() => beep('sawtooth', 165, 82, 0.2, stingerVol), 150);
                    } else if (theme === 'gameplay' && (prevTheme === 'boss' || prevTheme === 'victory')) {
                        [523, 659, 784].forEach((f, i) => setTimeout(() => beep('sine', f, f, 0.1, stingerVol), i * 60));
                    } else if (theme === 'victory') {
                        [784, 988, 1175, 1568].forEach((f, i) => setTimeout(() => beep('sine', f, f, 0.12, stingerVol), 2800 + i * 150));
                    }
                }
                this.masterGain = a.createGain();
                this.masterGain.gain.value = ctx.G.muted ? 0 : ctx.G.vol * 0.35;
                this.masterGain.connect(ctx.masterCompressor || a.destination);
                const th = this.themes[theme]; if (!th) return;
                const beatDur = (60 / th.bpm) / this.tempoMult;
                const loop = theme !== 'gameover' && theme !== 'victory';
                const schedVoices = () => {
                    if (this.stopped || !this.masterGain) return;
                    this.nodes = [];
                    let maxDur = 0;
                    const percBoost = this.intensity <= 2 ? 0.7 : this.intensity <= 4 ? 1 : this.intensity <= 7 ? 1.3 : 1.6;
                    for (const vn of ['bass', 'lead', 'harmony', 'arpeggio']) {
                        const voice = th[vn]; if (!voice) continue;
                        const iFactor = this.intensity <= 2 ? (vn === 'bass' || vn === 'harmony' ? 1 : vn === 'lead' ? 0.2 : 0)
                            : this.intensity <= 4 ? (vn === 'arpeggio' ? 0.3 : 1)
                            : this.intensity <= 7 ? (vn === 'arpeggio' ? 0.7 : 1) : 1;
                        if (iFactor <= 0) continue;
                        let offset = 0;
                        for (const n of voice.notes) {
                            if (n.f > 0) {
                                const o = a.createOscillator(), g = a.createGain();
                                o.type = voice.wave; o.frequency.value = n.f;
                                g.gain.setValueAtTime(voice.vol * (ctx.G.muted ? 0 : 1) * iFactor, a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + n.d * beatDur + 0.01);
                                if (vn === 'bass') {
                                    const filt = a.createBiquadFilter(); filt.type = 'lowpass';
                                    filt.frequency.setValueAtTime(250, a.currentTime + offset);
                                    filt.frequency.linearRampToValueAtTime(700, a.currentTime + offset + n.d * beatDur * 0.3);
                                    filt.frequency.linearRampToValueAtTime(180, a.currentTime + offset + n.d * beatDur);
                                    o.connect(filt).connect(g).connect(this.masterGain);
                                } else { o.connect(g).connect(this.masterGain); }
                                if (ctx.reverbNode && (vn === 'lead' || vn === 'harmony')) {
                                    const rvbSend = a.createGain(); rvbSend.gain.value = 0.08;
                                    g.connect(rvbSend); rvbSend.connect(ctx.reverbNode);
                                }
                                o.start(a.currentTime + offset); o.stop(a.currentTime + offset + n.d * beatDur + 0.02);
                                this.nodes.push(o);
                            }
                            offset += n.d * beatDur;
                        }
                        maxDur = Math.max(maxDur, offset);
                    }
                    if (th.percussion) {
                        let offset = 0;
                        for (const n of th.percussion.notes) {
                            if (n.f === -1) {
                                const o = a.createOscillator(), g = a.createGain();
                                o.frequency.setValueAtTime(150, a.currentTime + offset);
                                o.frequency.exponentialRampToValueAtTime(40, a.currentTime + offset + 0.08);
                                g.gain.setValueAtTime(th.percussion.vol * 1.8 * percBoost, a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + 0.15);
                                o.connect(g).connect(this.masterGain);
                                o.start(a.currentTime + offset); o.stop(a.currentTime + offset + 0.16);
                                this.nodes.push(o);
                                const ns = schedNoise(a.currentTime + offset, 0.06, th.percussion.vol * 0.6, 120, this.masterGain);
                                if (ns) this.nodes.push(ns);
                            }
                            else if (n.f === -2) {
                                const ns = schedNoise(a.currentTime + offset, 0.04, th.percussion.vol * 0.7, 9000, this.masterGain);
                                if (ns) this.nodes.push(ns);
                                const ns2 = schedNoise(a.currentTime + offset, 0.02, th.percussion.vol * 0.3, 3000, this.masterGain);
                                if (ns2) this.nodes.push(ns2);
                            }
                            else if (n.f === -3) {
                                const ns = schedNoise(a.currentTime + offset, 0.07, th.percussion.vol * 0.8, 2500, this.masterGain);
                                if (ns) this.nodes.push(ns);
                                const o = a.createOscillator(), g = a.createGain();
                                o.frequency.setValueAtTime(200, a.currentTime + offset);
                                o.frequency.exponentialRampToValueAtTime(80, a.currentTime + offset + 0.1);
                                g.gain.setValueAtTime(th.percussion.vol * 0.5, a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + 0.12);
                                o.connect(g).connect(this.masterGain);
                                o.start(a.currentTime + offset); o.stop(a.currentTime + offset + 0.13);
                                this.nodes.push(o);
                            }
                            offset += n.d * beatDur;
                        }
                        maxDur = Math.max(maxDur, offset);
                    }
                    if (loop) { this.loopId = setTimeout(() => { schedVoices(); }, maxDur * 1000 + 50); }
                };
                schedVoices();
            },
            stop() { this.stopped = true; clearTimeout(this.loopId); for (const n of this.nodes) try { n.stop(); } catch (e) {} this.nodes = []; if (this.masterGain) try { this.masterGain.disconnect(); } catch (e) {} this.playing = null; },
            setTempo(mult) { this.tempoMult = mult; if (this.playing) this.play(this.playing); },
            setMuted(m) { if (this.masterGain) this.masterGain.gain.value = m ? 0 : ctx.G.vol * 0.35; if (ctx.GalagaMusic) ctx.GalagaMusic.setMuted(m); },
            setIntensity(level) {
                this.intensity = level;
                const volMult = 1 + Math.min(level, 5) * 0.08;
                if (this.masterGain) this.masterGain.gain.value = ctx.G.muted ? 0 : ctx.G.vol * 0.35 * volMult;
            },
            addLayer(layerId, layerDef) {
                if (!this.layerGains) this.layerGains = {};
                const a = audio(); if (!a) return null;
                const gainNode = a.createGain();
                gainNode.gain.value = 0;
                gainNode.connect(this.masterGain || a.destination);
                this.layerGains[layerId] = { gain: gainNode, def: layerDef, active: false };
                return this.layerGains[layerId];
            },
            removeLayer(themeId, layerId) {
                if (!this.layerGains || !this.layerGains[layerId]) return;
                try { this.layerGains[layerId].gain.disconnect(); } catch (_) {}
                delete this.layerGains[layerId];
            },
            setLayerGain(themeId, layerId, value) {
                if (!this.layerGains || !this.layerGains[layerId]) return;
                this.layerGains[layerId].gain.gain.value = Math.min(0.04, value);
            },
            transpose(semitones) {
                if (!this.semitoneOffset) this.semitoneOffset = 0;
                this.semitoneOffset = semitones;
                if (this.playing) {
                    const wasPlaying = this.playing;
                    this.play(wasPlaying);
                }
            }
        };

        const GalagaMusic = {
            el: null,
            _playing: false,
            _playPending: null,
            _shouldPlay: false,
            _blocked: false,
            _retryAt: 0,
            _url: '/img/audio/galaga.mp3',
            _ensure() {
                if (this.el) return this.el;
                const a = document.createElement('audio');
                a.src = this._url;
                a.loop = true;
                a.preload = 'auto';
                a.volume = Math.max(0, Math.min(1, (ctx.G.vol || 0.3) * 0.7));
                a.addEventListener('playing', () => { if (this.el === a) this._playing = true; });
                a.addEventListener('pause', () => { if (this.el === a) this._playing = false; });
                a.addEventListener('error', () => {
                    if (this.el !== a) return;
                    this._playing = false;
                    this._playPending = null;
                    this._retryAt = Date.now() + 1000;
                    this.el = null;
                });
                this.el = a;
                return a;
            },
            play(fromGesture) {
                this._shouldPlay = true;
                if (ctx.G && ctx.G.muted) return;
                if (fromGesture) { this._blocked = false; this._retryAt = 0; }
                if (this._playing || (!fromGesture && this._playPending) || this._blocked || Date.now() < this._retryAt) return;
                const a = this._ensure();
                try {
                    a.currentTime = 0;
                    const pending = a.play();
                    if (!pending || typeof pending.then !== 'function') { this._playing = !a.paused; return; }
                    this._playPending = pending;
                    pending.then(() => {
                        if (this._playPending !== pending) return;
                        this._playPending = null;
                        if (!this._shouldPlay || (ctx.G && ctx.G.muted)) { try { a.pause(); a.currentTime = 0; } catch (_) {} return; }
                        this._playing = true;
                        this._blocked = false;
                    }).catch(err => {
                        if (this._playPending !== pending) return;
                        this._playPending = null;
                        this._playing = false;
                        this._blocked = !!(err && err.name === 'NotAllowedError');
                        this._retryAt = Date.now() + 1000;
                    });
                } catch (err) {
                    this._playing = false;
                    this._blocked = !!(err && err.name === 'NotAllowedError');
                    this._retryAt = Date.now() + 1000;
                }
            },
            resumeFromGesture() {
                if (this._shouldPlay && !(ctx.G && ctx.G.muted)) this.play(true);
            },
            stop() {
                this._shouldPlay = false;
                this._playPending = null;
                if (!this._playing && !this.el) return;
                const a = this._ensure();
                try { a.pause(); a.currentTime = 0; } catch (_) {}
                this._playing = false;
            },
            setMuted(m) {
                if (!this.el && !m) this._ensure();
                if (this.el) this.el.volume = m ? 0 : Math.max(0, Math.min(1, (ctx.G.vol || 0.3) * 0.7));
                if (m && this._playing) { try { this.el.pause(); } catch (_) {} this._playing = false; }
                else if (!m && this._shouldPlay && !this._playing) { this.play(); }
            }
        };
        ctx.GalagaMusic = GalagaMusic;
        ctx.MusicEngine = MusicEngine;
    };
})();
