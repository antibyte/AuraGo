(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createAudioMusic = function (ctx) {
        const motifs = {
            nebula: {
                root: 57, bpm: 124, wave: 'triangle',
                notes:  [12, 0, 7, 10, 12, 0, 15, 14, 12, 7, 10, 0, 7, 5, 3, 7],
                notesB: [15, 14, 12, 10, 12, 15, 19, 17, 15, 12, 10, 7, 8, 10, 12, 14],
                arp:    [0, 7, 12, 15, 12, 7, 10, 7],
                chords: [[0, 7, 15], [8, 12, 19], [5, 12, 17], [7, 10, 14]],
                bass:   [0, 8, 5, 7]
            },
            asteroid: {
                root: 50, bpm: 132, wave: 'sawtooth',
                notes:  [0, 12, 0, 3, 7, 0, 6, 7, 12, 0, 10, 7, 6, 3, 0, 3],
                notesB: [12, 12, 15, 12, 10, 7, 6, 7, 12, 15, 18, 15, 14, 10, 7, 3],
                arp:    [0, 3, 7, 12, 7, 6, 3, 0],
                chords: [[0, 7, 15], [3, 10, 15], [5, 12, 17], [1, 8, 13]],
                bass:   [0, 3, 5, 1]
            },
            crystal: {
                root: 62, bpm: 118, wave: 'sine',
                notes:  [12, 19, 24, 22, 19, 15, 14, 19, 12, 14, 19, 22, 24, 19, 15, 14],
                notesB: [24, 22, 19, 15, 17, 19, 22, 24, 26, 24, 22, 19, 17, 15, 14, 10],
                arp:    [12, 15, 19, 24, 19, 15, 14, 19],
                chords: [[0, 7, 14], [8, 15, 19], [5, 12, 17], [10, 14, 21]],
                bass:   [0, 8, 5, 10]
            },
            storm: {
                root: 52, bpm: 140, wave: 'square',
                notes:  [0, 7, 12, 7, 0, 10, 14, 10, 0, 7, 15, 14, 12, 10, 7, 3],
                notesB: [12, 15, 19, 15, 14, 10, 7, 10, 12, 15, 22, 19, 17, 15, 14, 10],
                arp:    [0, 7, 12, 15, 14, 12, 10, 7],
                chords: [[0, 7, 15], [10, 14, 22], [8, 12, 19], [7, 11, 19]],
                bass:   [0, 10, 8, 7]
            },
            blackhole: {
                root: 48, bpm: 108, wave: 'sine',
                notes:  [12, 0, 7, 0, 13, 12, 0, 7, 10, 0, 6, 0, 7, 6, 3, 0],
                notesB: [15, 13, 12, 7, 8, 7, 6, 3, 12, 10, 7, 6, 3, 1, 0, 7],
                arp:    [0, 6, 7, 12, 13, 12, 7, 6],
                chords: [[0, 7, 15], [1, 8, 13], [8, 12, 19], [7, 10, 18]],
                bass:   [0, 1, 8, 7]
            },
            void: {
                root: 45, bpm: 144, wave: 'triangle',
                notes:  [12, 7, 15, 14, 12, 3, 7, 10, 12, 19, 17, 15, 14, 10, 7, 0],
                notesB: [19, 15, 12, 15, 20, 19, 15, 14, 19, 22, 20, 19, 17, 15, 14, 11],
                arp:    [0, 7, 12, 15, 19, 15, 12, 7],
                chords: [[0, 7, 15], [8, 12, 20], [5, 12, 17], [7, 11, 19]],
                bass:   [0, 8, 5, 7]
            }
        };
        const hz = midi => 440 * Math.pow(2, (midi - 69) / 12);
        let timer = 0, nextTime = 0, step = 0, pending = null, paused = false;
        const layers = new Map();
        const MusicEngine = {
            themes: {}, playing: null, masterGain: null, tempoMult: 1, intensity: 3, semitoneOffset: 0,
            resolve(name) {
                if (name === 'title' || ctx.G.demoMode) return null;
                if (['boss', 'miniboss', 'deep_boss'].includes(name)) return (ctx.G.biome || 'nebula') + '_boss';
                if (name === 'gameplay') return ctx.G.biome || 'nebula';
                return name;
            },
            play(name) {
                const id = this.resolve(name);
                if (!id) { this.stop(); return; }
                if (id === this.playing || id === pending) return;
                const a = ctx.audio(); if (!a) return;
                this.masterGain = ctx.musicBus;
                if (ctx.GalagaMusic) ctx.GalagaMusic.stop();
                if (this.playing) pending = id;
                else { this.playing = id; nextTime = a.currentTime + 0.15; step = 0; }
                pump();
            },
            stop() { clearTimeout(timer); timer = 0; ctx.stopMusicVoices(); this.playing = null; pending = null; },
            setPaused(value) {
                if (value === paused) return;
                paused = value;
                if (value) { clearTimeout(timer); timer = 0; ctx.stopMusicVoices(); }
                else if (this.playing) { nextTime = (ctx.actx ? ctx.actx.currentTime : 0) + 0.05; pump(); }
            },
            setTempo(mult) { this.tempoMult = Math.max(0.8, Math.min(1.35, mult)); },
            setMuted(m) { ctx.G.muted = m; ctx.applyAudioSettings(); },
            setIntensity(value) { this.intensity = Math.max(0, Math.min(10, value)); },
            transpose(value) { this.semitoneOffset = value; },
            addLayer(id, def) { const layer = { def, gain: Math.min(0.04, def.vol || 0.03) }; layers.set(id, layer); return layer; },
            removeLayer(theme, id) { layers.delete(id); },
            setLayerGain(theme, id, gain) { if (layers.has(id)) layers.get(id).gain = Math.min(0.04, gain); }
        };
        for (const [id, motif] of Object.entries(motifs)) {
            MusicEngine.themes[id] = motif;
            MusicEngine.themes[id + '_boss'] = { ...motif, bpm: motif.bpm + 10, wave: motif.wave === 'sine' ? 'sawtooth' : motif.wave };
        }
        Object.assign(MusicEngine.themes, {
            gameplay: motifs.nebula,
            boss: { ...motifs.void, bpm: 152, wave: 'sawtooth' },
            challenge: {
                ...motifs.crystal, bpm: 144, wave: 'triangle',
                notes: [12, 16, 19, 24, 22, 19, 16, 12, 14, 17, 21, 24, 22, 19, 16, 14],
                chords: [[0, 7, 16], [5, 9, 17], [7, 11, 19], [0, 7, 16]],
                bass: [0, 5, 7, 0]
            },
            shop: {
                ...motifs.nebula, bpm: 92, wave: 'sine',
                notes: [12, 14, 15, 19, 17, 15, 14, 12, 10, 12, 14, 15, 14, 12, 10, 7],
                chords: [[0, 7, 14], [8, 12, 19], [5, 9, 16], [7, 10, 17]],
                bass: [0, 8, 5, 7]
            },
            victory: {
                ...motifs.crystal, root: 60, bpm: 116, wave: 'triangle',
                notes: [12, 16, 19, 24, 24, 26, 28, 24, 21, 19, 16, 19, 24, 19, 16, 12],
                chords: [[0, 7, 16], [5, 9, 17], [7, 11, 19], [0, 7, 16]],
                bass: [0, 5, 7, 0]
            },
            gameover: {
                ...motifs.blackhole, bpm: 84, wave: 'sine',
                notes: [12, 10, 8, 7, 5, 3, 2, 0, 7, 5, 3, 2, 0, -1, 0, 0]
            },
            gauntlet: { ...motifs.asteroid, bpm: 138, wave: 'sawtooth' },
            hyperdrive: { ...motifs.storm, bpm: 148, wave: 'sawtooth' },
            mirror: { ...motifs.crystal, bpm: 128, wave: 'triangle' },
            title: { bpm: 120 }
        });
        function sequence(time) {
            const m = MusicEngine, th = m.themes[m.playing] || motifs.nebula, beat = 60 / (th.bpm * m.tempoMult);
            const boss = m.playing.endsWith('_boss') || m.playing === 'boss';
            const quiet = m.playing === 'shop' || m.playing === 'gameover';
            const adaptive = !!ctx.settings.adaptiveMusic;
            const root = th.root + (adaptive ? m.semitoneOffset : 0);
            const stepInBar = step % 16, bar = Math.floor(step / 16);
            const useSectionB = (bar % 4 >= 2) && th.notesB;
            const phrase = useSectionB ? th.notesB : th.notes;
            const note = phrase[stepInBar];
            const bassRoot = (th.bass || [0, 8, 5, 7])[bar % 4];
            const chord = (th.chords || [[0, 7, 15]])[bar % 4];
            const leadGain = quiet ? 0.085 : (boss ? 0.135 : 0.12);
            const tone = (wave, pitch, length, volume, pan, fm, opts) =>
                ctx.synthTone(wave, hz(pitch), hz(pitch), length, volume, pan, time, ctx.musicBus, fm, opts);

            // 1. Lead Melody with dynamic filter envelope & stereo polish
            if ((!quiet || stepInBar % 2 === 0) && note !== undefined && note >= 0) {
                const leadFm = th.wave === 'sine' ? 2 : (boss ? 1.5 : 0);
                const accent = (stepInBar % 4 === 0) ? 1.12 : 0.92;
                tone(th.wave, root + note, beat * (stepInBar % 4 === 0 ? 0.38 : 0.28), leadGain * accent, ctx.W * 0.64, leadFm, {
                    cut0: quiet ? 2400 : (boss ? 5200 : 4200),
                    cut1: quiet ? 900 : 1400,
                    q: boss ? 2.0 : 1.3,
                    detune: (step % 2 === 0 ? 4 : -4),
                    reverb: quiet ? 0.22 : 0.12
                });
            }

            // 2. Counter-Arpeggio & Harmonic Chords
            if (!quiet && th.arp && stepInBar % 2 === 1 && (boss || m.intensity >= 3)) {
                const arpNote = th.arp[(stepInBar >> 1) % th.arp.length];
                tone('triangle', root + arpNote, beat * 0.22, 0.048, ctx.W * 0.30, 0, {
                    cut0: 3600, cut1: 1200, reverb: 0.14
                });
            }
            if (stepInBar === 0 || (stepInBar === 8 && !quiet)) {
                const c1 = chord[1] !== undefined ? chord[1] : 7;
                const c2 = chord[2] !== undefined ? chord[2] : 15;
                const padLen = quiet ? beat * 1.8 : beat * 1.15;
                const padGain = quiet ? 0.042 : 0.048;
                tone('triangle', root - 12 + c1, padLen, padGain, ctx.W * 0.38, 0, { cut0: 1900, cut1: 750, detune: -6, reverb: 0.18 });
                tone('sine', root - 12 + c2, padLen, padGain * 0.85, ctx.W * 0.62, 1.5, { cut0: 2200, cut1: 900, detune: 6, reverb: 0.18 });
            }

            // 3. Driving Synthwave / Arcade Bassline (Downbeat sub + Offbeat octave pluck)
            if (stepInBar % 4 === 0) {
                tone(boss ? 'sawtooth' : 'triangle', root - 24 + bassRoot, beat * 0.78, quiet ? 0.16 : 0.21, ctx.W * 0.46, 0, {
                    cut0: boss ? 1600 : 1100, cut1: 280, q: 2.0
                });
                if (!quiet) {
                    // Punchy synthesized kick drum (pitch sweep + transient click)
                    ctx.synthTone('sine', 128, 36, 0.14, 0.25, ctx.W * 0.5, time, ctx.musicBus, 0, { exp: true });
                    ctx.synthTone('triangle', 420, 75, 0.028, 0.055, ctx.W * 0.5, time, ctx.musicBus, 0, { exp: true });
                }
            } else if (!quiet && stepInBar % 4 === 2) {
                tone('sawtooth', root - 12 + bassRoot, beat * 0.28, 0.095, ctx.W * 0.54, 0, {
                    cut0: 1800, cut1: 420, q: 1.6
                });
            } else if (boss && stepInBar === 14) {
                // Syncopated double-kick before bar turnaround on boss tracks
                ctx.synthTone('sine', 118, 38, 0.11, 0.20, ctx.W * 0.5, time, ctx.musicBus, 0, { exp: true });
            }

            // 4. Snare, Hi-Hat Groove & 4-Bar Turnaround Fills
            const isFillStep = !quiet && (bar % 4 === 3) && (stepInBar === 13 || stepInBar === 15);
            if (!quiet && (stepInBar % 8 === 4 || isFillStep)) {
                const snVol = isFillStep ? 0.09 : 0.135;
                ctx.schedNoise(time, isFillStep ? 0.08 : 0.115, snVol, 2800, ctx.musicBus, ctx.W * 0.52, {
                    cut0: 5400, cut1: 1400, q: 1.1, reverb: 0.12
                });
                ctx.synthTone('triangle', isFillStep ? 240 : 205, 110, 0.055, snVol * 0.55, ctx.W * 0.5, time, ctx.musicBus, 0, { exp: true });
            }
            if (!quiet && (stepInBar % (boss ? 1 : 2) === 0 || stepInBar % 4 === 2)) {
                const openHat = stepInBar % 4 === 2;
                ctx.schedNoise(time, openHat ? 0.042 : 0.022, openHat ? 0.042 : 0.026, 9000, ctx.musicBus, ctx.W * 0.62, {
                    filterType: 'highpass', cut0: 6800, cut1: 9600
                });
            } else if (m.playing === 'shop' && stepInBar % 4 === 2) {
                // Gentle chill shaker on shop theme
                ctx.schedNoise(time, 0.03, 0.022, 7500, ctx.musicBus, ctx.W * 0.58, { filterType: 'bandpass', cut0: 6500, cut1: 8200 });
            }

            // 5. Boss Aggression Stabs & High-Intensity Octave Shimmer
            if (boss && stepInBar % 4 === 2 && note >= 0) {
                tone('square', root + note - 12, beat * 0.18, 0.075, ctx.W * 0.25, 0, { cut0: 2600, cut1: 800 });
            }
            if (adaptive && m.intensity > 6 && stepInBar % 4 === 0 && note >= 0) {
                tone('triangle', root + note + 12, beat * 0.42, 0.065, ctx.W * 0.72, 1.5, { cut0: 5500, cut1: 2200, reverb: 0.18 });
            }

            // 6. Biome Ambient Pad Layers
            if (stepInBar === 0) for (const layer of layers.values()) {
                const n = layer.def.notes && layer.def.notes[bar % layer.def.notes.length];
                if (n && n.f > 0) {
                    ctx.synthTone(layer.def.wave, n.f, n.f, Math.min(2, beat * 3.5), layer.gain, ctx.W * 0.5, time, ctx.musicBus, 0, {
                        cut0: 2400, cut1: 900, detune: 5, reverb: 0.22
                    });
                }
            }
            nextTime += beat / 4; step++;
        }
        function pump() {
            clearTimeout(timer); timer = 0;
            const a = ctx.actx;
            if (!a || ctx.state.disposed || !MusicEngine.playing || paused) return;
            if (a.state === 'running') {
                if (nextTime < a.currentTime - 0.1) nextTime = a.currentTime + 0.02;
                while (nextTime < a.currentTime + 0.12) {
                    if (step % 16 === 0 && pending) { MusicEngine.playing = pending; pending = null; }
                    try {
                        sequence(nextTime);
                    } catch (_) {
                        nextTime += 0.125;
                        step++;
                    }
                }
            }
            timer = setTimeout(pump, 25);
        }

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
                a.volume = Math.max(0, Math.min(1, (ctx.G.vol ?? 0.3) * (ctx.settings.musicVol ?? 70) / 100));
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
                if (this.el) this.el.volume = m ? 0 : Math.max(0, Math.min(1, (ctx.G.vol ?? 0.3) * (ctx.settings.musicVol ?? 70) / 100));
                if (m && this._playing) { try { this.el.pause(); } catch (_) {} this._playing = false; }
                else if (!m && this._shouldPlay && !this._playing) { this.play(); }
            }
        };
        ctx.GalagaMusic = GalagaMusic;
        ctx.MusicEngine = MusicEngine;
    };
})();
